package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"gitlab-analytics/internal/gitlab"
	"gitlab-analytics/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// SyncService 同步服务
type SyncService struct {
	db *gorm.DB
	mu sync.Mutex
}

// SyncResult 同步结果
type SyncResult struct {
	RunID   uint   `json:"runId"`
	Started bool   `json:"started"`
	Message string `json:"message"`
}

// NewSyncService 创建同步服务
func NewSyncService(db *gorm.DB) *SyncService { return &SyncService{db: db} }

// StartAsync 异步启动同步
func (s *SyncService) StartAsync(full bool) SyncResult {
	if !s.mu.TryLock() {
		return SyncResult{Started: false, Message: "已有同步任务正在运行"}
	}
	mode := "incremental"
	if full {
		mode = "full"
	}
	run := model.SyncRun{Mode: mode, Status: "running", StartedAt: time.Now()}
	if err := s.db.Create(&run).Error; err != nil {
		s.mu.Unlock()
		return SyncResult{Message: err.Error()}
	}
	go func() { defer s.mu.Unlock(); s.run(context.Background(), &run, full) }()
	return SyncResult{RunID: run.ID, Started: true, Message: "同步任务已启动"}
}

// run 运行同步
func (s *SyncService) run(ctx context.Context, run *model.SyncRun, full bool) {
	finish := time.Now()
	updates := map[string]any{"status": "success", "finished_at": &finish}
	if err := s.sync(ctx, run, full); err != nil {
		updates["status"] = "failed"
		updates["error_message"] = err.Error()
	}
	updates["projects_scanned"] = run.ProjectsScanned
	updates["commits_synced"] = run.CommitsSynced
	s.db.Model(&model.SyncRun{}).Where("id = ?", run.ID).Updates(updates)
}

// sync 同步数据
func (s *SyncService) sync(ctx context.Context, run *model.SyncRun, full bool) error {
	var setting model.Setting
	if err := s.db.First(&setting, 1).Error; err != nil {
		return fmt.Errorf("尚未保存 GitLab 配置")
	}
	client, err := gitlab.New(setting.GitLabURL, setting.GitLabToken)
	if err != nil {
		return err
	}
	rootIDs, err := parseIDs(setting.RootGroupIDs)
	if err != nil {
		return err
	}
	if len(rootIDs) == 0 {
		groups, err := client.ListTopLevelGroups(ctx)
		if err != nil {
			return fmt.Errorf("读取全部顶级项目组: %w", err)
		}
		for _, group := range groups {
			rootIDs = append(rootIDs, group.ID)
		}
		if len(rootIDs) == 0 {
			return fmt.Errorf("当前 Token 没有可访问的顶级项目组")
		}
	}
	var since *time.Time
	if !full && setting.LastSuccessAt != nil {
		backfill := setting.LastSuccessAt.Add(-15 * 24 * time.Hour)
		since = &backfill
	}
	for _, rootID := range rootIDs {
		root, err := client.GetGroup(ctx, rootID)
		if err != nil {
			return err
		}
		if err := s.upsertGroup(root, rootID); err != nil {
			return err
		}
		projects, err := client.ListProjects(ctx, rootID)
		if err != nil {
			return fmt.Errorf("读取项目组 %s: %w", root.FullPath, err)
		}
		for _, remote := range projects {
			localProject, err := s.upsertProject(remote, rootID)
			if err != nil {
				return err
			}
			run.ProjectsScanned++
			commits, err := client.ListCommits(ctx, remote.ID, since)
			if err != nil {
				return fmt.Errorf("读取项目 %s 的提交: %w", remote.PathWithNamespace, err)
			}
			for _, remoteCommit := range commits {
				// 【修改点 1】：接收 upsertCommit 的返回值，只在新插入时才算作“同步了提交”
				isNew, err := s.upsertCommit(localProject.ID, remoteCommit)
				if err != nil {
					return err
				}
				if isNew {
					run.CommitsSynced++
				}
			}
		}
	}
	now := time.Now()
	return s.db.Model(&model.Setting{}).Where("id = ?", setting.ID).Update("last_success_at", now).Error
}

// upsertGroup 插入或更新组
func (s *SyncService) upsertGroup(remote gitlab.Group, rootID int64) error {
	group := model.Group{GitLabID: remote.ID, RootGitLabID: rootID, Name: remote.Name, FullPath: remote.FullPath, WebURL: remote.WebURL}
	return s.db.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "git_lab_id"}}, DoUpdates: clause.AssignmentColumns([]string{"root_git_lab_id", "name", "full_path", "web_url", "updated_at"})}).Create(&group).Error
}

// upsertProject 插入或更新项目
func (s *SyncService) upsertProject(remote gitlab.Project, rootID int64) (model.Project, error) {
	group := model.Group{GitLabID: remote.Namespace.ID, RootGitLabID: rootID, Name: remote.Namespace.Name, FullPath: remote.Namespace.FullPath, WebURL: ""}
	if err := s.db.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "git_lab_id"}}, DoUpdates: clause.AssignmentColumns([]string{"root_git_lab_id", "name", "full_path", "updated_at"})}).Create(&group).Error; err != nil {
		return model.Project{}, err
	}
	if err := s.db.Where("git_lab_id = ?", remote.Namespace.ID).First(&group).Error; err != nil {
		return model.Project{}, err
	}
	project := model.Project{GitLabID: remote.ID, GroupID: &group.ID, Name: remote.Name, PathWithNS: remote.PathWithNamespace, WebURL: remote.WebURL, DefaultBranch: remote.DefaultBranch, Archived: remote.Archived}
	if err := s.db.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "git_lab_id"}}, DoUpdates: clause.AssignmentColumns([]string{"group_id", "name", "path_with_ns", "web_url", "default_branch", "archived", "updated_at"})}).Create(&project).Error; err != nil {
		return model.Project{}, err
	}
	if err := s.db.Where("git_lab_id = ?", remote.ID).First(&project).Error; err != nil {
		return model.Project{}, err
	}
	return project, nil
}

// upsertCommit 插入或更新提交
// 【修改点 2】：返回 (bool, error)，bool 代表是否是新插入的提交
func (s *SyncService) upsertCommit(projectID uint, remote gitlab.Commit) (bool, error) {
	user, err := s.resolveUser(remote.AuthorName, remote.AuthorEmail)
	if err != nil {
		return false, err
	}
	var userID *uint
	if user.ID != 0 {
		userID = &user.ID
	}
	commit := model.Commit{ProjectID: projectID, UserID: userID, SHA: remote.ID, ShortSHA: remote.ShortID, Title: remote.Title, Message: remote.Message, AuthorName: remote.AuthorName, AuthorEmail: strings.ToLower(remote.AuthorEmail), AuthoredAt: remote.AuthoredDate, CommittedAt: remote.CommittedDate, Additions: remote.Stats.Additions, Deletions: remote.Stats.Deletions, TotalChanges: remote.Stats.Total, IsMerge: len(remote.ParentIDs) > 1, WebURL: remote.WebURL}

	// 1. 先查一下这条记录是否存在
	var count int64
	s.db.Model(&model.Commit{}).Where("project_id = ? AND sha = ?", projectID, remote.ID).Count(&count)

	// 2. 执行 UPSERT (如果存在则更新，不存在则插入)
	err = s.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "project_id"}, {Name: "sha"}},
		DoUpdates: clause.AssignmentColumns([]string{"user_id", "short_sha", "title", "message", "author_name", "author_email", "authored_at", "committed_at", "additions", "deletions", "total_changes", "is_merge", "web_url", "updated_at"}),
	}).Create(&commit).Error

	// 3. 如果 count == 0，说明这是新数据
	return count == 0, err
}

// resolveUser 解析用户
func (s *SyncService) resolveUser(name, email string) (model.User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return model.User{}, nil
	}
	var alias model.UserAlias
	if err := s.db.Preload("User").Where("email = ?", email).First(&alias).Error; err == nil {
		return alias.User, nil
	} else if err != gorm.ErrRecordNotFound {
		return model.User{}, err
	}
	user := model.User{Name: name, Email: email, Username: strings.Split(email, "@")[0], IsBot: strings.Contains(strings.ToLower(name+email), "bot")}
	if err := s.db.Where("email = ?", email).FirstOrCreate(&user).Error; err != nil {
		return model.User{}, err
	}
	return user, nil
}

// parseIDs 解析ID列表
func parseIDs(value string) ([]int64, error) {
	var ids []int64
	for _, raw := range strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == '\n' || r == ' ' || r == ';' }) {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || id < 1 {
			return nil, fmt.Errorf("无效的项目组 ID: %q", raw)
		}
		ids = append(ids, id)
	}
	return ids, nil
}
