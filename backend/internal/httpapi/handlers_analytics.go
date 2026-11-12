package httpapi

import (
	"encoding/csv"
	"net/http"
	"strconv"
	"time"

	"gitlab-analytics/internal/model"

	"github.com/gin-gonic/gin"
)

// metricSummary 指标摘要
type metricSummary struct {
	Commits      int64 `json:"commits"`
	Additions    int64 `json:"additions"`
	Deletions    int64 `json:"deletions"`
	Contributors int64 `json:"contributors"`
	Projects     int64 `json:"projects"`
}

// trendPoint 趋势点
type trendPoint struct {
	Date      string `json:"date"`
	Commits   int64  `json:"commits"`
	Additions int64  `json:"additions"`
	Deletions int64  `json:"deletions"`
}

// contributorRow 贡献者行
type contributorRow struct {
	UserID      uint       `json:"userId"`
	Name        string     `json:"name"`
	Email       string     `json:"email"`
	CommitCount int64      `json:"commitCount"`
	Additions   int64      `json:"additions"`
	Deletions   int64      `json:"deletions"`
	Projects    int64      `json:"projects"`
	FirstCommit *time.Time `json:"firstCommit"`
	LastCommit  *time.Time `json:"lastCommit"`
}

// projectRow 项目行
type projectRow struct {
	ProjectID    uint   `json:"projectId"`
	Name         string `json:"name"`
	Path         string `json:"path"`
	GroupName    string `json:"groupName"`
	CommitCount  int64  `json:"commitCount"`
	Additions    int64  `json:"additions"`
	Deletions    int64  `json:"deletions"`
	Contributors int64  `json:"contributors"`
}

// dashboard 获取仪表板数据
func (s *Server) dashboard(c *gin.Context) {
	f, err := parseFilters(c)
	if err != nil {
		badRequest(c, err.Error())
		return
	}

	var summary metricSummary
	if err := filteredCommits(s.db, f).
		Select("COUNT(*) AS commits, COALESCE(SUM(commits.additions),0) AS additions, COALESCE(SUM(commits.deletions),0) AS deletions, COUNT(DISTINCT commits.user_id) AS contributors, COUNT(DISTINCT commits.project_id) AS projects").
		Scan(&summary).Error; err != nil {
		internalError(c, err)
		return
	}

	var trend []trendPoint
	if err := filteredCommits(s.db, f).
		Select("DATE_FORMAT(commits.authored_at, '%Y-%m-%d') AS date, COUNT(*) AS commits, COALESCE(SUM(commits.additions),0) AS additions, COALESCE(SUM(commits.deletions),0) AS deletions").
		Group("date").
		Order("date").
		Scan(&trend).Error; err != nil {
		internalError(c, err)
		return
	}

	var contributors []contributorRow
	if err := filteredCommits(s.db, f).
		Joins("LEFT JOIN users ON users.id = commits.user_id").
		Select("COALESCE(commits.user_id,0) AS user_id, COALESCE(users.name, commits.author_name) AS name, COALESCE(users.email, commits.author_email) AS email, COUNT(*) AS commit_count, COALESCE(SUM(commits.additions),0) AS additions, COALESCE(SUM(commits.deletions),0) AS deletions, COUNT(DISTINCT commits.project_id) AS projects, MIN(commits.authored_at) AS first_commit, MAX(commits.authored_at) AS last_commit").
		Group("commits.user_id, users.name, users.email, commits.author_name, commits.author_email").
		Order("additions DESC, commit_count DESC").
		Limit(8).
		Scan(&contributors).Error; err != nil {
		internalError(c, err)
		return
	}

	var projects []projectRow
	if err := filteredCommits(s.db, f).
		Select("projects.id AS project_id, projects.name AS name, projects.path_with_ns AS path, COALESCE(`groups`.name,'未分组') AS group_name, COUNT(*) AS commit_count, COALESCE(SUM(commits.additions),0) AS additions, COALESCE(SUM(commits.deletions),0) AS deletions, COUNT(DISTINCT commits.user_id) AS contributors").
		Group("projects.id, projects.name, projects.path_with_ns, `groups`.name").
		Order("additions DESC, commit_count DESC").
		Limit(8).
		Scan(&projects).Error; err != nil {
		internalError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"summary": summary, "trend": trend, "topContributors": contributors, "topProjects": projects})
}

// contributions 获取贡献数据
func (s *Server) contributions(c *gin.Context) {
	f, err := parseFilters(c)
	if err != nil {
		badRequest(c, err.Error())
		return
	}
	page, size := pageParams(c)

	var total int64
	if err := filteredCommits(s.db, f).
		Joins("LEFT JOIN users ON users.id = commits.user_id").
		Select("COUNT(DISTINCT COALESCE(commits.user_id, 0), commits.author_email)").
		Scan(&total).Error; err != nil {
		internalError(c, err)
		return
	}

	var rows []contributorRow
	if err := filteredCommits(s.db, f).
		Joins("LEFT JOIN users ON users.id = commits.user_id").
		Select("COALESCE(commits.user_id,0) AS user_id, COALESCE(users.name, commits.author_name) AS name, COALESCE(users.email, commits.author_email) AS email, COUNT(*) AS commit_count, COALESCE(SUM(commits.additions),0) AS additions, COALESCE(SUM(commits.deletions),0) AS deletions, COUNT(DISTINCT commits.project_id) AS projects, MIN(commits.authored_at) AS first_commit, MAX(commits.authored_at) AS last_commit").
		Group("commits.user_id, users.name, users.email, commits.author_name, commits.author_email").
		Order("additions DESC, commit_count DESC").
		Offset((page - 1) * size).
		Limit(size).
		Scan(&rows).Error; err != nil {
		internalError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"items": rows, "page": page, "pageSize": size, "total": total})
}

// commits 获取提交数据
func (s *Server) commits(c *gin.Context) {
	f, err := parseFilters(c)
	if err != nil {
		badRequest(c, err.Error())
		return
	}
	page, size := pageParams(c)

	var total int64
	if err := filteredCommits(s.db, f).Count(&total).Error; err != nil {
		internalError(c, err)
		return
	}

	var commits []model.Commit
	if err := filteredCommits(s.db, f).
		Preload("Project.Group").
		Preload("User").
		Order("commits.authored_at DESC").
		Offset((page - 1) * size).
		Limit(size).
		Find(&commits).Error; err != nil {
		internalError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"items": commits, "page": page, "pageSize": size, "total": total})
}

// exportCommits 导出提交数据
func (s *Server) exportCommits(c *gin.Context) {
	f, err := parseFilters(c)
	if err != nil {
		badRequest(c, err.Error())
		return
	}

	var commits []model.Commit
	if err := filteredCommits(s.db, f).
		Preload("Project.Group").
		Preload("User").
		Order("commits.authored_at DESC").
		Limit(10000).
		Find(&commits).Error; err != nil {
		internalError(c, err)
		return
	}

	c.Header("Content-Disposition", "attachment; filename=gitlab-contributions.csv")
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Writer.Write([]byte("\xEF\xBB\xBF"))
	writer := csv.NewWriter(c.Writer)
	defer writer.Flush()
	_ = writer.Write([]string{"提交时间", "贡献者", "邮箱", "项目组", "项目", "SHA", "标题", "新增行", "删除行", "净变化", "GitLab 链接"})
	for _, item := range commits {
		name := item.AuthorName
		if item.User != nil {
			name = item.User.Name
		}
		group := ""
		if item.Project.Group != nil {
			group = item.Project.Group.Name
		}
		_ = writer.Write([]string{item.AuthoredAt.Format("2006-01-02 15:04:05"), name, item.AuthorEmail, group, item.Project.PathWithNS, item.ShortSHA, item.Title, strconv.Itoa(item.Additions), strconv.Itoa(item.Deletions), strconv.Itoa(item.Additions - item.Deletions), item.WebURL})
	}
}

// syncRuns 获取同步运行记录
func (s *Server) syncRuns(c *gin.Context) {
	var runs []model.SyncRun
	if err := s.db.Order("started_at DESC").Limit(50).Find(&runs).Error; err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": runs})
}

// syncStatus 获取同步状态
func (s *Server) syncStatus(c *gin.Context) {
	var run model.SyncRun
	_ = s.db.Order("started_at DESC").First(&run).Error
	var setting model.Setting
	_ = s.db.First(&setting, 1).Error
	c.JSON(http.StatusOK, gin.H{"lastRun": run, "lastSuccessAt": setting.LastSuccessAt})
}

// startSync 启动同步
func (s *Server) startSync(c *gin.Context) {
	var input struct {
		Full bool `json:"full"`
	}
	_ = c.ShouldBindJSON(&input)
	result := s.syncer.StartAsync(input.Full)
	status := http.StatusAccepted
	if !result.Started {
		status = http.StatusConflict
	}
	c.JSON(status, result)
}
