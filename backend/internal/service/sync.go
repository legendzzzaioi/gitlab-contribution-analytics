package service

import (
	"context"
	"fmt"
	"sort"
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

// groupMemberSnapshot 保存 Group 成员及共享关系，供 Group 和 Project 权限计算使用。
type groupMemberSnapshot struct {
	DirectMembers map[int64][]gitlab.Member
	AllMembers    map[int64][]gitlab.Member
	InvitedGroups map[int64][]int64
}

// NewSyncService 创建同步服务
func NewSyncService(db *gorm.DB) *SyncService {
	return &SyncService{db: db}
}

// StartAsync 异步启动同步
func (s *SyncService) StartAsync(full bool) SyncResult {
	if !s.mu.TryLock() {
		return SyncResult{
			Started: false,
			Message: "已有同步任务正在运行",
		}
	}

	mode := "incremental"
	if full {
		mode = "full"
	}

	run := model.SyncRun{
		Mode:      mode,
		Status:    "running",
		StartedAt: time.Now(),
	}

	if err := s.db.Create(&run).Error; err != nil {
		s.mu.Unlock()
		return SyncResult{Message: err.Error()}
	}

	go func() {
		defer s.mu.Unlock()
		s.run(context.Background(), &run, full)
	}()

	return SyncResult{
		RunID:   run.ID,
		Started: true,
		Message: "同步任务已启动",
	}
}

// run 运行同步任务
func (s *SyncService) run(
	ctx context.Context,
	run *model.SyncRun,
	full bool,
) {
	defer func() {
		if r := recover(); r != nil {
			run.Status = "failed"
			run.ErrorMessage = fmt.Sprintf(
				"同步任务发生异常: %v",
				r,
			)
		}

		finishedAt := time.Now()
		run.FinishedAt = &finishedAt

		if run.Status == "running" {
			run.Status = "success"
		}

		if err := s.db.Model(&model.SyncRun{}).
			Where("id = ?", run.ID).
			Updates(map[string]interface{}{
				"status":           run.Status,
				"finished_at":      finishedAt,
				"projects_scanned": run.ProjectsScanned,
				"commits_synced":   run.CommitsSynced,
				"error_message":    run.ErrorMessage,
			}).Error; err != nil {
			fmt.Printf(
				"更新同步运行记录失败，runID=%d: %v\n",
				run.ID,
				err,
			)
		}
	}()

	if err := s.sync(ctx, run, full); err != nil {
		run.Status = "failed"
		run.ErrorMessage = err.Error()
		return
	}

	// 只有全部同步成功后才更新最后成功时间。
	finishedAt := time.Now()

	if err := s.db.Model(&model.Setting{}).
		Where("id = ?", 1).
		Update("last_success_at", finishedAt).Error; err != nil {
		run.Status = "failed"
		run.ErrorMessage = fmt.Sprintf(
			"更新最后同步成功时间失败: %v",
			err,
		)
		return
	}

	run.Status = "success"
}

// sync 同步 GitLab 数据
func (s *SyncService) sync(
	ctx context.Context,
	run *model.SyncRun,
	full bool,
) error {
	// 1. 读取配置
	var setting model.Setting
	if err := s.db.First(&setting, 1).Error; err != nil {
		return fmt.Errorf("读取 GitLab 配置失败: %w", err)
	}

	// 2. 创建 GitLab 客户端
	client, err := gitlab.New(
		setting.GitLabURL,
		setting.GitLabToken,
	)
	if err != nil {
		return fmt.Errorf("创建 GitLab 客户端失败: %w", err)
	}

	// 3. 解析根 Group ID
	rootIDs, err := parseIDs(setting.RootGroupIDs)
	if err != nil {
		return err
	}

	// 未配置根 Group 时，获取 Token 可访问的顶级 Group。
	if len(rootIDs) == 0 {
		remoteGroups, err := client.ListTopLevelGroups(ctx)
		if err != nil {
			return fmt.Errorf("读取顶级 Group 失败: %w", err)
		}

		for _, group := range remoteGroups {
			rootIDs = append(rootIDs, group.ID)
		}

		if len(rootIDs) == 0 {
			return fmt.Errorf("当前 Token 没有可访问的顶级 Group")
		}
	}

	// 4. 确定提交同步起始时间
	var since *time.Time

	if !full && setting.LastSuccessAt != nil {
		// 增量同步向前回溯 15 天，避免遗漏延迟数据。
		backfill := setting.LastSuccessAt.Add(-15 * 24 * time.Hour)
		since = &backfill
	}

	// 5. 记录本次同步实际获取到的 Group 和 Project。
	// 只有全部同步成功后，才根据这两个集合清理过期数据。
	syncedGroupIDs := make(map[int64]struct{})
	syncedProjectIDs := make(map[int64]struct{})

	// 防止多个根 Group 存在重叠范围时重复同步同一个 Project。
	processedProjectIDs := make(map[int64]struct{})

	// 6. 遍历根 Group
	for _, rootID := range rootIDs {
		if err := ctx.Err(); err != nil {
			return err
		}

		// 获取根 Group
		root, err := client.GetGroup(ctx, rootID)
		if err != nil {
			return fmt.Errorf(
				"获取根 Group %d 失败: %w",
				rootID,
				err,
			)
		}

		// 每个根 Group 建立自己的 Group 树。
		groups := make(map[int64]gitlab.Group)
		visited := make(map[int64]bool)

		// 递归同步根 Group 和所有子 Group。
		if err := s.syncGroupTree(
			ctx,
			client,
			root,
			rootID,
			visited,
			groups,
		); err != nil {
			return fmt.Errorf(
				"同步 Group 树失败，rootGroupID=%d: %w",
				rootID,
				err,
			)
		}

		// 记录本次同步获取到的 Group。
		for groupID := range groups {
			syncedGroupIDs[groupID] = struct{}{}
		}

		// 7. 同步所有 Group 的成员权限。
		// 返回各 Group 的直接成员，用于计算 Project 继承权限。
		groupMembers, err := s.syncGroupMembers(
			ctx,
			client,
			groups,
		)
		if err != nil {
			return fmt.Errorf(
				"同步 Group 成员失败，rootGroupID=%d: %w",
				rootID,
				err,
			)
		}

		// 8. 获取整个 Group 树下的项目。
		// 按 GitLab ID 去重，避免父组接口包含子组项目时重复处理。
		projectMap := make(map[int64]gitlab.Project)

		groupIDs := make([]int64, 0, len(groups))
		for groupID := range groups {
			groupIDs = append(groupIDs, groupID)
		}

		sort.Slice(groupIDs, func(i, j int) bool {
			return groupIDs[i] < groupIDs[j]
		})

		for _, groupID := range groupIDs {
			if err := ctx.Err(); err != nil {
				return err
			}

			projects, err := client.ListProjects(ctx, groupID)
			if err != nil {
				return fmt.Errorf(
					"读取 Group %d 的项目失败: %w",
					groupID,
					err,
				)
			}

			for _, project := range projects {
				projectMap[project.ID] = project
			}
		}

		// 转换为切片并排序，保证同步顺序稳定。
		projects := make([]gitlab.Project, 0, len(projectMap))

		for _, project := range projectMap {
			// 多个根 Group 范围重叠时，避免重复处理。
			if _, exists := processedProjectIDs[project.ID]; exists {
				continue
			}

			processedProjectIDs[project.ID] = struct{}{}
			projects = append(projects, project)
		}

		sort.Slice(projects, func(i, j int) bool {
			return projects[i].ID < projects[j].ID
		})

		// 9. 保存 Project 基础信息。
		for _, remoteProject := range projects {
			if err := ctx.Err(); err != nil {
				return err
			}

			// Project 所属 Namespace 必须已经存在于 Group 树中。
			if _, exists := groups[remoteProject.Namespace.ID]; !exists {
				return fmt.Errorf(
					"Project %d 的 Namespace Group %d 不在已同步的 Group 树中",
					remoteProject.ID,
					remoteProject.Namespace.ID,
				)
			}

			// 获取项目详细信息，包括统计信息
			detail, err := client.GetProject(ctx, remoteProject.ID)
			if err != nil {
				return fmt.Errorf("获取项目 %d (%s) 统计信息失败: %w",
					remoteProject.ID, remoteProject.PathWithNamespace, err)
			}

			remoteProject.Statistics = detail.Statistics

			// 保存 Project 基础信息。
			if _, err := s.upsertProject(remoteProject); err != nil {
				return fmt.Errorf(
					"保存 Project %d 失败: %w",
					remoteProject.ID,
					err,
				)
			}

			syncedProjectIDs[remoteProject.ID] = struct{}{}
			run.ProjectsScanned++
		}

		// 10. 同步所有 Project 成员权限。
		if err := s.syncProjectMembers(
			ctx,
			client,
			projects,
			groups,
			groupMembers,
		); err != nil {
			return fmt.Errorf(
				"同步 Project 成员失败，rootGroupID=%d: %w",
				rootID,
				err,
			)
		}

		// 11. 同步项目提交记录。
		for _, remoteProject := range projects {
			if err := ctx.Err(); err != nil {
				return err
			}

			var localProject model.Project

			if err := s.db.
				Where("git_lab_id = ?", remoteProject.ID).
				First(&localProject).Error; err != nil {
				return fmt.Errorf(
					"查询本地 Project %d 失败: %w",
					remoteProject.ID,
					err,
				)
			}

			// 获取所有分支的提交记录，默认分支请用 ListCommits
			commits, err := client.ListAllBranchCommits(
				ctx,
				remoteProject.ID,
				since,
			)
			if err != nil {
				return fmt.Errorf(
					"读取 Project %s 的提交失败: %w",
					remoteProject.PathWithNamespace,
					err,
				)
			}

			for _, remoteCommit := range commits {
				if err := ctx.Err(); err != nil {
					return err
				}

				isNew, err := s.upsertCommit(
					localProject.ID,
					remoteCommit,
				)
				if err != nil {
					return fmt.Errorf(
						"保存 Project %d 的 Commit %s 失败: %w",
						remoteProject.ID,
						remoteCommit.ID,
						err,
					)
				}

				if isNew {
					run.CommitsSynced++
				}
			}
		}
	}

	// 12. 所有根 Group 的数据均同步成功后，再清理过期数据。
	if err := ctx.Err(); err != nil {
		return err
	}

	if err := s.cleanupStaleData(
		syncedGroupIDs,
		syncedProjectIDs,
	); err != nil {
		return fmt.Errorf("清理过期数据失败: %w", err)
	}

	// 统一由 run() 在全部同步成功后更新最后成功时间。
	return nil
}

// cleanupStaleData 清理本次同步范围内已经不存在的数据。
//
// 注意：必须在所有 Group、Project、Member 同步成功后调用。
// 使用事务保证清理过程整体成功或整体回滚。
func (s *SyncService) cleanupStaleData(
	syncedGroupIDs map[int64]struct{},
	syncedProjectIDs map[int64]struct{},
) error {
	groupIDs := make([]int64, 0, len(syncedGroupIDs))
	for id := range syncedGroupIDs {
		groupIDs = append(groupIDs, id)
	}
	sort.Slice(groupIDs, func(i, j int) bool {
		return groupIDs[i] < groupIDs[j]
	})

	projectIDs := make([]int64, 0, len(syncedProjectIDs))
	for id := range syncedProjectIDs {
		projectIDs = append(projectIDs, id)
	}
	sort.Slice(projectIDs, func(i, j int) bool {
		return projectIDs[i] < projectIDs[j]
	})

	if len(groupIDs) == 0 {
		return fmt.Errorf("本次同步没有获取到任何 Group，取消过期数据清理")
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		// 1. 查询已经不存在的 Project。
		projectQuery := tx.Unscoped()

		if len(projectIDs) > 0 {
			projectQuery = projectQuery.Where(
				"git_lab_id NOT IN ?",
				projectIDs,
			)
		}

		var staleProjects []model.Project
		if err := projectQuery.Find(&staleProjects).Error; err != nil {
			return fmt.Errorf("查询过期 Project 失败: %w", err)
		}

		staleProjectLocalIDs := make([]uint, 0, len(staleProjects))
		for _, project := range staleProjects {
			staleProjectLocalIDs = append(
				staleProjectLocalIDs,
				project.ID,
			)
		}

		// 2. 清理过期 Project 的成员权限。
		projectMemberQuery := tx.Unscoped().Where(
			"target_type = ?",
			"project",
		)

		if len(projectIDs) > 0 {
			projectMemberQuery = projectMemberQuery.Where(
				"target_git_lab_id NOT IN ?",
				projectIDs,
			)
		}

		if err := projectMemberQuery.Delete(&model.Member{}).Error; err != nil {
			return fmt.Errorf("清理过期 Project 成员权限失败: %w", err)
		}

		// 3. 清理过期 Group 的成员权限。
		if err := tx.Unscoped().
			Where(
				"target_type = ? AND target_git_lab_id NOT IN ?",
				"group",
				groupIDs,
			).
			Delete(&model.Member{}).Error; err != nil {
			return fmt.Errorf("清理过期 Group 成员权限失败: %w", err)
		}

		// 4. 删除过期 Project 对应的 Commit。
		if len(staleProjectLocalIDs) > 0 {
			if err := tx.Unscoped().
				Where("project_id IN ?", staleProjectLocalIDs).
				Delete(&model.Commit{}).Error; err != nil {
				return fmt.Errorf("清理过期 Project 的 Commit 失败: %w", err)
			}
		}

		// 5. 删除过期 Project。
		staleProjectDelete := tx.Unscoped()

		if len(projectIDs) > 0 {
			staleProjectDelete = staleProjectDelete.Where(
				"git_lab_id NOT IN ?",
				projectIDs,
			)
		}

		if err := staleProjectDelete.Delete(&model.Project{}).Error; err != nil {
			return fmt.Errorf("删除过期 Project 失败: %w", err)
		}

		// 6. 删除过期 Group。
		if err := tx.Unscoped().
			Where("git_lab_id NOT IN ?", groupIDs).
			Delete(&model.Group{}).Error; err != nil {
			return fmt.Errorf("删除过期 Group 失败: %w", err)
		}

		return nil
	})
}

// upsertGroup 插入或更新组
func (s *SyncService) upsertGroup(
	remote gitlab.Group,
	rootGitLabID int64,
) error {
	group := model.Group{
		GitLabID:       remote.ID,
		RootGitLabID:   rootGitLabID,
		ParentGitLabID: remote.ParentID,
		Name:           remote.Name,
		FullPath:       remote.FullPath,
		WebURL:         remote.WebURL,
		Description:    remote.Description,
		CreationDate:   remote.CreatedAt,
	}

	return s.db.
		Where("git_lab_id = ?", remote.ID).
		Assign(group).
		FirstOrCreate(&group).Error
}

// upsertProject 插入或更新项目
func (s *SyncService) upsertProject(
	remote gitlab.Project,
) (model.Project, error) {
	var group model.Group

	if err := s.db.
		Where("git_lab_id = ?", remote.Namespace.ID).
		First(&group).Error; err != nil {
		return model.Project{}, fmt.Errorf(
			"查询 Project Namespace Group 失败，projectID=%d groupID=%d: %w",
			remote.ID,
			remote.Namespace.ID,
			err,
		)
	}

	project := model.Project{
		GitLabID:       remote.ID,
		GroupID:        &group.ID,
		Name:           remote.Name,
		PathWithNS:     remote.PathWithNamespace,
		WebURL:         remote.WebURL,
		DefaultBranch:  remote.DefaultBranch,
		Archived:       remote.Archived,
		Description:    remote.Description,
		CreationDate:   remote.CreatedAt,
		StorageSize:    remote.Statistics.StorageSize,
		RepositorySize: remote.Statistics.RepositorySize,
		CommitCount:    remote.Statistics.CommitCount,
	}

	if err := s.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "git_lab_id"},
		},
		DoUpdates: clause.AssignmentColumns([]string{
			"group_id",
			"name",
			"path_with_ns",
			"web_url",
			"default_branch",
			"archived",
			"description",
			"creation_date",
			"storage_size",
			"repository_size",
			"commit_count",
			"updated_at",
		}),
	}).Create(&project).Error; err != nil {
		return model.Project{}, fmt.Errorf(
			"保存 Project %d 失败: %w",
			remote.ID,
			err,
		)
	}

	if err := s.db.
		Where("git_lab_id = ?", remote.ID).
		First(&project).Error; err != nil {
		return model.Project{}, err
	}

	return project, nil
}

// upsertCommit 插入或更新提交
func (s *SyncService) upsertCommit(
	projectID uint,
	remote gitlab.Commit,
) (bool, error) {
	user, err := s.resolveUser(
		remote.AuthorName,
		remote.AuthorEmail,
	)
	if err != nil {
		return false, err
	}

	var userID *uint
	if user.ID != 0 {
		userID = &user.ID
	}

	commit := model.Commit{
		ProjectID:    projectID,
		UserID:       userID,
		SHA:          remote.ID,
		ShortSHA:     remote.ShortID,
		Title:        remote.Title,
		Message:      remote.Message,
		AuthorName:   remote.AuthorName,
		AuthorEmail:  strings.ToLower(remote.AuthorEmail),
		AuthoredAt:   remote.AuthoredDate,
		CommittedAt:  remote.CommittedDate,
		Additions:    remote.Stats.Additions,
		Deletions:    remote.Stats.Deletions,
		TotalChanges: remote.Stats.Total,
		IsMerge:      len(remote.ParentIDs) > 1,
		WebURL:       remote.WebURL,
	}

	var count int64

	if err := s.db.Model(&model.Commit{}).
		Where("project_id = ? AND sha = ?", projectID, remote.ID).
		Count(&count).Error; err != nil {
		return false, fmt.Errorf(
			"检查 Commit 是否存在失败，projectID=%d sha=%s: %w",
			projectID,
			remote.ID,
			err,
		)
	}

	err = s.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "project_id"},
			{Name: "sha"},
		},
		DoUpdates: clause.AssignmentColumns([]string{
			"user_id",
			"short_sha",
			"title",
			"message",
			"author_name",
			"author_email",
			"authored_at",
			"committed_at",
			"additions",
			"deletions",
			"total_changes",
			"is_merge",
			"web_url",
			"updated_at",
		}),
	}).Create(&commit).Error

	return count == 0, err
}

// resolveUser 解析用户
func (s *SyncService) resolveUser(
	name string,
	email string,
) (model.User, error) {
	email = strings.ToLower(strings.TrimSpace(email))

	if email == "" {
		return model.User{}, nil
	}

	var alias model.UserAlias

	if err := s.db.Preload("User").
		Where("email = ?", email).
		First(&alias).Error; err == nil {
		return alias.User, nil
	} else if err != gorm.ErrRecordNotFound {
		return model.User{}, err
	}

	user := model.User{
		Name:     name,
		Email:    email,
		Username: strings.Split(email, "@")[0],
		IsBot:    strings.Contains(strings.ToLower(name+email), "bot"),
	}

	if err := s.db.
		Where("email = ?", email).
		FirstOrCreate(&user).Error; err != nil {
		return model.User{}, err
	}

	return user, nil
}

// parseIDs 解析并去重 ID 列表
func parseIDs(value string) ([]int64, error) {
	var ids []int64
	seen := make(map[int64]struct{})

	for _, raw := range strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' ||
			r == '\n' ||
			r == ' ' ||
			r == ';' ||
			r == '\t' ||
			r == '\r'
	}) {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || id < 1 {
			return nil, fmt.Errorf("无效的项目组 ID: %q", raw)
		}

		if _, exists := seen[id]; exists {
			continue
		}

		seen[id] = struct{}{}
		ids = append(ids, id)
	}

	return ids, nil
}

// syncGroupMembers 同步 Group 成员
// 返回所有 Group 的直接成员映射，供 Project 继承权限计算使用。

func (s *SyncService) syncGroupMembers(
	ctx context.Context,
	client *gitlab.Client,
	groups map[int64]gitlab.Group,
) (*groupMemberSnapshot, error) {
	groupIDs := make([]int64, 0, len(groups))
	for id := range groups {
		groupIDs = append(groupIDs, id)
	}
	sort.Slice(groupIDs, func(i, j int) bool {
		return groupIDs[i] < groupIDs[j]
	})

	snapshot := &groupMemberSnapshot{
		DirectMembers: make(map[int64][]gitlab.Member),
		AllMembers:    make(map[int64][]gitlab.Member),
		InvitedGroups: make(map[int64][]int64),
	}

	// 第一轮：获取所有 Group 的直接成员和共享关系。
	for _, groupID := range groupIDs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		members, err := client.ListMembers(ctx, "groups", groupID)
		if err != nil {
			return nil, fmt.Errorf(
				"读取 Group %d 直接成员失败: %w",
				groupID,
				err,
			)
		}
		snapshot.DirectMembers[groupID] = members

		// 获取共享到当前 Group 的来源 Group。
		sharedGroups, err := client.ListInvitedGroups(ctx, groupID)
		if err != nil {
			return nil, fmt.Errorf(
				"读取 Group %d 共享来源失败: %w",
				groupID,
				err,
			)
		}

		for _, sharedGroup := range sharedGroups {
			snapshot.InvitedGroups[groupID] = append(
				snapshot.InvitedGroups[groupID],
				sharedGroup.ID,
			)
		}

		sort.Slice(
			snapshot.InvitedGroups[groupID],
			func(i, j int) bool {
				return snapshot.InvitedGroups[groupID][i] <
					snapshot.InvitedGroups[groupID][j]
			},
		)
	}

	// 第二轮：获取当前树之外的共享来源 Group 的直接成员。
	sharedSourceIDs := make(map[int64]struct{})

	for _, sourceIDs := range snapshot.InvitedGroups {
		for _, sourceID := range sourceIDs {
			if _, exists := snapshot.DirectMembers[sourceID]; exists {
				continue
			}
			sharedSourceIDs[sourceID] = struct{}{}
		}
	}

	sourceIDs := make([]int64, 0, len(sharedSourceIDs))
	for id := range sharedSourceIDs {
		sourceIDs = append(sourceIDs, id)
	}
	sort.Slice(sourceIDs, func(i, j int) bool {
		return sourceIDs[i] < sourceIDs[j]
	})

	for _, sourceID := range sourceIDs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		members, err := client.ListMembers(ctx, "groups", sourceID)
		if err != nil {
			return nil, fmt.Errorf(
				"读取共享来源 Group %d 直接成员失败: %w",
				sourceID,
				err,
			)
		}

		snapshot.DirectMembers[sourceID] = members
	}

	// 第三轮：获取当前 Group 树中所有 Group 的有效成员。
	for _, groupID := range groupIDs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		allMembers, err := client.ListAllMembers(ctx, "groups", groupID)
		if err != nil {
			return nil, fmt.Errorf(
				"读取 Group %d 有效成员失败: %w",
				groupID,
				err,
			)
		}

		snapshot.AllMembers[groupID] = allMembers
	}

	// 第四轮：获取当前树之外的共享来源 Group 的有效成员。
	for _, sourceID := range sourceIDs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		allMembers, err := client.ListAllMembers(ctx, "groups", sourceID)
		if err != nil {
			return nil, fmt.Errorf(
				"读取共享来源 Group %d 有效成员失败: %w",
				sourceID,
				err,
			)
		}

		snapshot.AllMembers[sourceID] = allMembers
	}

	// 第五轮：生成并保存 Group 成员快照。
	for _, groupID := range groupIDs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		ancestorIDs := getAncestorGroupIDs(groupID, groups)

		records := buildMemberRecords(
			"group",
			groupID,
			snapshot.DirectMembers[groupID],
			snapshot.AllMembers[groupID],
			ancestorIDs,
			snapshot.DirectMembers,
			snapshot.InvitedGroups,
		)

		if err := s.saveMemberSnapshot("group", groupID, records); err != nil {
			return nil, fmt.Errorf(
				"保存 Group %d 成员失败: %w",
				groupID,
				err,
			)
		}
	}

	return snapshot, nil
}

// getAncestorGroupIDs 查找 Group 祖先
func getAncestorGroupIDs(
	groupID int64,
	groups map[int64]gitlab.Group,
) []int64 {
	var result []int64

	current, ok := groups[groupID]
	if !ok {
		return result
	}

	parentID := current.ParentID

	for parentID > 0 {
		result = append(result, parentID)

		parent, exists := groups[parentID]
		if !exists {
			break
		}

		parentID = parent.ParentID
	}

	return result
}

// buildMemberRecords 转换成员记录
func buildMemberRecords(
	targetType string,
	targetID int64,
	directMembers []gitlab.Member,
	allMembers []gitlab.Member,
	ancestorIDs []int64,
	groupDirectMembers map[int64][]gitlab.Member,
	invitedGroups map[int64][]int64,
) []model.Member {
	// 当前目标的直接成员。
	directMap := make(map[int64]struct{}, len(directMembers))
	for _, member := range directMembers {
		directMap[member.ID] = struct{}{}
	}

	// 缓存每个来源 Group 的直接成员 ID。
	groupDirectMemberMaps := make(map[int64]map[int64]struct{})

	getGroupDirectMemberMap := func(groupID int64) map[int64]struct{} {
		if memberMap, ok := groupDirectMemberMaps[groupID]; ok {
			return memberMap
		}

		memberMap := make(map[int64]struct{})

		for _, member := range groupDirectMembers[groupID] {
			memberMap[member.ID] = struct{}{}
		}

		groupDirectMemberMaps[groupID] = memberMap
		return memberMap
	}

	// 按优先级生成候选来源 Group。
	sourceGroupIDs := make([]int64, 0)
	seen := make(map[int64]struct{})

	appendSource := func(groupID int64) {
		if groupID <= 0 {
			return
		}

		if _, exists := seen[groupID]; exists {
			return
		}

		seen[groupID] = struct{}{}
		sourceGroupIDs = append(sourceGroupIDs, groupID)
	}

	// Group 目标：先检查直接共享给当前 Group 的来源。
	if targetType == "group" {
		for _, sourceID := range invitedGroups[targetID] {
			appendSource(sourceID)
		}
	}

	// 再检查所属 Group 和所有祖先 Group。
	// ancestorIDs 按距离当前目标由近到远排列。
	for _, ancestorID := range ancestorIDs {
		appendSource(ancestorID)

		// 祖先 Group 自身通过共享获得权限的来源。
		for _, sourceID := range invitedGroups[ancestorID] {
			appendSource(sourceID)
		}
	}

	records := make([]model.Member, 0, len(allMembers))

	for _, effective := range allMembers {
		record := model.Member{
			TargetType:     targetType,
			TargetGitLabID: targetID,
			UserID:         effective.ID,
			UserName:       effective.Name,
			Username:       effective.Username,
			AccessLevel:    effective.AccessLevel,
			GrantedAt:      effective.CreatedAt,
			ExpiresAt:      effective.ExpiresAt,
		}

		// 当前目标直接授权，不需要记录继承来源。
		if _, exists := directMap[effective.ID]; exists {
			record.Inherited = false
			records = append(records, record)
			continue
		}

		record.Inherited = true

		// 只查找真正直接拥有该用户的来源 Group。
		// 不使用 Group 的有效成员，否则会把中间继承层级误认为授权来源。
		for _, sourceGroupID := range sourceGroupIDs {
			memberMap := getGroupDirectMemberMap(sourceGroupID)

			if _, exists := memberMap[effective.ID]; !exists {
				continue
			}

			record.SourceTargetType = "group"
			record.SourceTargetGitLabID = sourceGroupID
			break
		}

		records = append(records, record)
	}

	return records
}

// saveMemberSnapshot 保存成员快照
func (s *SyncService) saveMemberSnapshot(
	targetType string,
	targetID int64,
	members []model.Member,
) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		// 先保存当前成员。
		if len(members) > 0 {
			if err := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{
					{Name: "target_type"},
					{Name: "target_git_lab_id"},
					{Name: "user_id"},
				},
				DoUpdates: clause.AssignmentColumns([]string{
					"user_name",
					"username",
					"access_level",
					"inherited",
					"source_target_type",
					"source_target_git_lab_id",
					"granted_at",
					"expires_at",
					"updated_at",
				}),
			}).Create(&members).Error; err != nil {
				return fmt.Errorf("保存成员失败: %w", err)
			}
		}

		// 再删除本次 API 已经确认不存在的成员。
		userIDs := make([]int64, 0, len(members))
		for _, member := range members {
			userIDs = append(userIDs, member.UserID)
		}

		query := tx.Unscoped().Where(
			"target_type = ? AND target_git_lab_id = ?",
			targetType,
			targetID,
		)

		if len(userIDs) > 0 {
			query = query.Where("user_id NOT IN ?", userIDs)
		}

		if err := query.Delete(&model.Member{}).Error; err != nil {
			return fmt.Errorf("清理失效成员失败: %w", err)
		}

		return nil
	})
}

// syncProjectMembers 同步 Project 成员
func (s *SyncService) syncProjectMembers(
	ctx context.Context,
	client *gitlab.Client,
	projects []gitlab.Project,
	groups map[int64]gitlab.Group,
	groupMembers *groupMemberSnapshot,
) error {
	if groupMembers == nil {
		return fmt.Errorf("Group 成员快照为空")
	}

	for _, project := range projects {
		if err := ctx.Err(); err != nil {
			return err
		}

		directMembers, err := client.ListMembers(
			ctx,
			"projects",
			project.ID,
		)
		if err != nil {
			return fmt.Errorf(
				"读取 Project %d 直接成员失败: %w",
				project.ID,
				err,
			)
		}

		allMembers, err := client.ListAllMembers(
			ctx,
			"projects",
			project.ID,
		)
		if err != nil {
			return fmt.Errorf(
				"读取 Project %d 有效成员失败: %w",
				project.ID,
				err,
			)
		}

		// 项目所属 Group 和所有祖先 Group。
		ancestorIDs := []int64{project.Namespace.ID}
		ancestorIDs = append(
			ancestorIDs,
			getAncestorGroupIDs(project.Namespace.ID, groups)...,
		)

		records := buildMemberRecords(
			"project",
			project.ID,
			directMembers,
			allMembers,
			ancestorIDs,
			groupMembers.DirectMembers,
			groupMembers.InvitedGroups,
		)

		if err := s.saveMemberSnapshot(
			"project",
			project.ID,
			records,
		); err != nil {
			return fmt.Errorf(
				"保存 Project %d 成员失败: %w",
				project.ID,
				err,
			)
		}
	}

	return nil
}

// syncGroupTree 递归同步 Group 树
//
// remote:       当前 GitLab Group
// rootGitLabID: 最顶层配置的根 Group ID
// visited:      防止重复访问和循环递归
// groups:       保存本次同步获取的全部 Group
func (s *SyncService) syncGroupTree(
	ctx context.Context,
	client *gitlab.Client,
	remote gitlab.Group,
	rootGitLabID int64,
	visited map[int64]bool,
	groups map[int64]gitlab.Group,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	// 防止重复处理。
	if visited[remote.ID] {
		return nil
	}

	visited[remote.ID] = true

	// 保存当前 Group。
	if err := s.upsertGroup(remote, rootGitLabID); err != nil {
		return fmt.Errorf(
			"保存 Group 失败，groupID=%d: %w",
			remote.ID,
			err,
		)
	}

	// 保存到当前 Group 树。
	groups[remote.ID] = remote

	// 获取当前 Group 的所有直接子 Group。
	subGroups, err := client.ListSubGroups(ctx, remote.ID)
	if err != nil {
		return fmt.Errorf(
			"获取 Group %d 的子 Group 失败: %w",
			remote.ID,
			err,
		)
	}

	// 递归同步子 Group。
	for _, subGroup := range subGroups {
		if subGroup.ParentID != remote.ID {
			return fmt.Errorf(
				"Group 父级关系异常: groupID=%d, parentID=%d, expectedParentID=%d",
				subGroup.ID,
				subGroup.ParentID,
				remote.ID,
			)
		}

		if err := s.syncGroupTree(
			ctx,
			client,
			subGroup,
			rootGitLabID,
			visited,
			groups,
		); err != nil {
			return err
		}
	}

	return nil
}
