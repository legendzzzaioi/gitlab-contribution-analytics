package httpapi

import (
	"context"
	"net/http"
	"strings"
	"time"

	"gitlab-analytics/internal/gitlab"
	"gitlab-analytics/internal/model"

	"github.com/gin-gonic/gin"
	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
)

// settingsInput 设置输入
type settingsInput struct {
	GitLabURL    string `json:"gitlabUrl"`
	GitLabToken  string `json:"gitlabToken"`
	RootGroupIDs string `json:"rootGroupIds"`
	SyncCron     string `json:"syncCron"`
}

// getSettings 获取设置
func (s *Server) getSettings(c *gin.Context) {
	var setting model.Setting
	if err := s.db.First(&setting, 1).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"gitlabUrl": "", "tokenConfigured": false, "rootGroupIds": "", "syncCron": s.cfg.SyncCron})
		return
	}
	c.JSON(http.StatusOK, settingResponse(setting))
}

// updateSettings 更新设置
func (s *Server) updateSettings(c *gin.Context) {
	var input settingsInput
	if err := c.ShouldBindJSON(&input); err != nil {
		badRequest(c, "配置格式不正确")
		return
	}
	input.GitLabURL = strings.TrimRight(strings.TrimSpace(input.GitLabURL), "/")
	input.RootGroupIDs = strings.TrimSpace(input.RootGroupIDs)
	if input.SyncCron == "" {
		input.SyncCron = s.cfg.SyncCron
	}
	if _, err := cron.ParseStandard(input.SyncCron); err != nil {
		badRequest(c, "同步 Cron 表达式无效，例如 0 */12 * * *")
		return
	}
	var existing model.Setting
	if err := s.db.First(&existing, 1).Error; err != nil && err != gorm.ErrRecordNotFound {
		internalError(c, err)
		return
	}
	if input.GitLabToken == "" {
		input.GitLabToken = existing.GitLabToken
	}
	setting := model.Setting{}
	if existing.ID == 0 {
		setting = model.Setting{ID: 1, GitLabURL: input.GitLabURL, GitLabToken: input.GitLabToken, RootGroupIDs: input.RootGroupIDs, SyncCron: input.SyncCron}
		if err := s.db.Create(&setting).Error; err != nil {
			internalError(c, err)
			return
		}
	} else {
		updates := map[string]any{"git_lab_url": input.GitLabURL, "git_lab_token": input.GitLabToken, "root_group_ids": input.RootGroupIDs, "sync_cron": input.SyncCron}
		if err := s.db.Model(&existing).Updates(updates).Error; err != nil {
			internalError(c, err)
			return
		}
		setting = existing
		setting.GitLabURL, setting.GitLabToken, setting.RootGroupIDs, setting.SyncCron = input.GitLabURL, input.GitLabToken, input.RootGroupIDs, input.SyncCron
	}
	if s.scheduler != nil {
		if err := s.scheduler.Update(input.SyncCron); err != nil {
			internalError(c, err)
			return
		}
	}
	c.JSON(http.StatusOK, settingResponse(setting))
}

// testSettings 测试设置
func (s *Server) testSettings(c *gin.Context) {
	var input settingsInput
	_ = c.ShouldBindJSON(&input)
	var current model.Setting
	_ = s.db.First(&current, 1).Error
	if strings.TrimSpace(input.GitLabURL) == "" {
		input.GitLabURL = current.GitLabURL
	}
	if strings.TrimSpace(input.GitLabToken) == "" {
		input.GitLabToken = current.GitLabToken
	}
	client, err := gitlab.New(input.GitLabURL, input.GitLabToken)
	if err == nil {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Second)
		defer cancel()
		err = client.Test(ctx)
	}
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "GitLab 连接成功，访问令牌有效"})
}

// listGroups 获取组列表
func (s *Server) listGroups(c *gin.Context) {
	var groups []model.Group
	if err := s.db.Where("git_lab_id = root_git_lab_id").Order("name").Find(&groups).Error; err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": groups})
}

// listProjects 获取项目列表
func (s *Server) listProjects(c *gin.Context) {
	var projects []model.Project
	q := s.db.Preload("Group").Order("path_with_ns")
	if group := c.Query("groupId"); group != "" {
		q = q.Joins("JOIN `groups` ON `groups`.id = projects.group_id").Where("`groups`.root_git_lab_id = ?", group)
	}
	if err := q.Find(&projects).Error; err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": projects})
}

// listUsers 获取用户列表
func (s *Server) listUsers(c *gin.Context) {
	var users []model.User
	if err := s.db.Order("name").Find(&users).Error; err != nil {
		internalError(c, err)
		return
	}
	var aliases []model.UserAlias
	_ = s.db.Order("email").Find(&aliases).Error
	aliasesByUser := map[uint][]model.UserAlias{}
	for _, a := range aliases {
		aliasesByUser[a.UserID] = append(aliasesByUser[a.UserID], a)
	}
	c.JSON(http.StatusOK, gin.H{"items": users, "aliases": aliasesByUser})
}

// updateUser 更新用户
func (s *Server) updateUser(c *gin.Context) {
	var input struct {
		Name     string `json:"name"`
		Username string `json:"username"`
		IsBot    bool   `json:"isBot"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		badRequest(c, "用户信息格式不正确")
		return
	}
	var user model.User
	if err := s.db.First(&user, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "未找到贡献者"})
		return
	}
	user.Name = strings.TrimSpace(input.Name)
	user.Username = strings.TrimSpace(input.Username)
	user.IsBot = input.IsBot
	if user.Name == "" {
		badRequest(c, "姓名不能为空")
		return
	}
	if err := s.db.Save(&user).Error; err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, user)
}

// createAlias 创建别名
func (s *Server) createAlias(c *gin.Context) {
	var input struct {
		Email string `json:"email"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		badRequest(c, "别名格式不正确")
		return
	}
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	if !strings.Contains(input.Email, "@") {
		badRequest(c, "请输入有效的邮箱")
		return
	}
	var user model.User
	if err := s.db.First(&user, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "未找到贡献者"})
		return
	}
	alias := model.UserAlias{UserID: user.ID, Email: input.Email}
	if err := s.db.Create(&alias).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"message": "该邮箱别名已经存在"})
		return
	}
	if err := s.db.Model(&model.Commit{}).Where("author_email = ?", input.Email).Update("user_id", user.ID).Error; err != nil {
		internalError(c, err)
		return
	}
	alias.User = user
	c.JSON(http.StatusCreated, alias)
}

// deleteAlias 删除别名
func (s *Server) deleteAlias(c *gin.Context) {
	var alias model.UserAlias
	if err := s.db.First(&alias, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "未找到邮箱别名"})
		return
	}
	if err := s.db.Delete(&alias).Error; err != nil {
		internalError(c, err)
		return
	}
	var fallback model.User
	if err := s.db.Where("email = ?", alias.Email).First(&fallback).Error; err == nil {
		if err := s.db.Model(&model.Commit{}).Where("author_email = ?", alias.Email).Update("user_id", fallback.ID).Error; err != nil {
			internalError(c, err)
			return
		}
	} else if err := s.db.Model(&model.Commit{}).Where("author_email = ?", alias.Email).Update("user_id", nil).Error; err != nil {
		internalError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
