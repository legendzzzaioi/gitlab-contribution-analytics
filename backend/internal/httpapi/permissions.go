package httpapi

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"gitlab-analytics/internal/model"

	"github.com/gin-gonic/gin"
)

// UserPermissionUser 用户权限列表
type UserPermissionUser struct {
	UserID       int64  `json:"userId"`
	UserName     string `json:"userName"`
	Username     string `json:"username"`
	GroupCount   int64  `json:"groupCount"`
	ProjectCount int64  `json:"projectCount"`
}

// UserPermissionResource 用户权限资源
type UserPermissionResource struct {
	TargetGitLabID   int64  `json:"targetGitlabId"`
	TargetName       string `json:"targetName"`
	TargetPath       string `json:"targetPath"`
	TargetURL        string `json:"targetUrl"`
	AccessLevel      int    `json:"accessLevel"`
	Inherited        bool   `json:"inherited"`
	SourceTargetName string `json:"sourceTargetName"`

	CreationDate time.Time `json:"creationDate"`
	Description  string    `json:"description"`

	// 项目专属字段
	CommitCount int64 `json:"commitCount,omitempty"`
	StorageSize int64 `json:"storageSize,omitempty"`
}

// getPermissionsTree 获取权限树
func (s *Server) getPermissionsTree(c *gin.Context) {
	var groups []model.Group
	if err := s.db.Order("full_path ASC").Find(&groups).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": err.Error(),
		})
		return
	}

	var projects []model.Project
	if err := s.db.Preload("Group").
		Order("path_with_ns ASC").
		Find(&projects).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": err.Error(),
		})
		return
	}

	var members []model.Member
	if err := s.db.Find(&members).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"groups":   groups,
		"projects": projects,
		"members":  members,
	})
}

// getUsersWithPermissions 获取所有拥有权限记录的用户
func (s *Server) getUsersWithPermissions(c *gin.Context) {
	search := strings.TrimSpace(c.Query("search"))

	var users []UserPermissionUser

	q := s.db.Model(&model.Member{}).
		Select(`
			user_id AS user_id,
			MAX(user_name) AS user_name,
			MAX(username) AS username,
			COUNT(DISTINCT CASE
				WHEN target_type = 'group' THEN target_git_lab_id
			END) AS group_count,
			COUNT(DISTINCT CASE
				WHEN target_type = 'project' THEN target_git_lab_id
			END) AS project_count
		`).
		Group("user_id").
		Order("user_name ASC")

	if search != "" {
		keyword := "%" + search + "%"
		q = q.Where(
			"user_name LIKE ? OR username LIKE ?",
			keyword,
			keyword,
		)
	}

	if err := q.Scan(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "查询用户列表失败",
		})
		return
	}

	if users == nil {
		users = make([]UserPermissionUser, 0)
	}

	c.JSON(http.StatusOK, gin.H{
		"users": users,
	})
}

// getUserPermissions 获取用户权限
func (s *Server) getUserPermissions(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("userId"), 10, 64)
	if err != nil || userID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "无效的用户 ID",
		})
		return
	}

	var members []model.Member
	if err := s.db.
		Where("user_id = ?", userID).
		Order("target_type ASC, access_level DESC").
		Find(&members).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "查询用户权限失败",
		})
		return
	}

	groupIDs := make([]int64, 0)
	projectIDs := make([]int64, 0)

	for _, m := range members {
		// 当前权限目标
		switch m.TargetType {
		case "group":
			groupIDs = append(groupIDs, m.TargetGitLabID)
		case "project":
			projectIDs = append(projectIDs, m.TargetGitLabID)
		}

		// 仅收集继承权限的来源
		if !m.Inherited {
			continue
		}

		switch m.SourceTargetType {
		case "group":
			groupIDs = append(groupIDs, m.SourceTargetGitLabID)
		case "project":
			projectIDs = append(projectIDs, m.SourceTargetGitLabID)
		}
	}

	groupMap := make(map[int64]model.Group)
	projectMap := make(map[int64]model.Project)

	if len(groupIDs) > 0 {
		var groups []model.Group
		if err := s.db.Where("git_lab_id IN ?", groupIDs).Find(&groups).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "查询群组信息失败",
			})
			return
		}

		for _, group := range groups {
			groupMap[group.GitLabID] = group
		}
	}

	if len(projectIDs) > 0 {
		var projects []model.Project
		if err := s.db.Where("git_lab_id IN ?", projectIDs).Find(&projects).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "查询项目信息失败",
			})
			return
		}

		for _, project := range projects {
			projectMap[project.GitLabID] = project
		}
	}

	// 根据来源类型和 GitLab ID 获取继承来源名称
	getSourceTargetName := func(m model.Member) string {
		if !m.Inherited {
			return ""
		}

		switch m.SourceTargetType {
		case "group":
			if group, ok := groupMap[m.SourceTargetGitLabID]; ok {
				return group.FullPath
			}

		case "project":
			if project, ok := projectMap[m.SourceTargetGitLabID]; ok {
				return project.PathWithNS
			}
		}

		return ""
	}

	groups := make([]UserPermissionResource, 0)
	projects := make([]UserPermissionResource, 0)

	for _, m := range members {
		item := UserPermissionResource{
			TargetGitLabID:   m.TargetGitLabID,
			AccessLevel:      m.AccessLevel,
			Inherited:        m.Inherited,
			SourceTargetName: getSourceTargetName(m),
		}

		switch m.TargetType {
		case "group":
			if group, ok := groupMap[m.TargetGitLabID]; ok {
				item.TargetName = group.Name
				item.TargetPath = group.FullPath
				item.TargetURL = group.WebURL
				item.CreationDate = group.CreationDate
				item.Description = group.Description
			}
			groups = append(groups, item)

		case "project":
			if project, ok := projectMap[m.TargetGitLabID]; ok {
				item.TargetName = project.Name
				item.TargetPath = project.PathWithNS
				item.TargetURL = project.WebURL
				item.CreationDate = project.CreationDate
				item.Description = project.Description
				item.CommitCount = project.CommitCount
				item.StorageSize = project.StorageSize
			}
			projects = append(projects, item)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"groups":   groups,
		"projects": projects,
	})
}
