package httpapi

import (
	"fmt"
	"strconv"
	"time"

	"gitlab-analytics/internal/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// filters 过滤条件
type filters struct {
	From, To          *time.Time
	GroupID           int64
	ProjectID, UserID uint
}

// parseFilters 解析过滤条件
func parseFilters(c *gin.Context) (filters, error) {
	f := filters{}
	parseDate := func(key string, end bool) (*time.Time, error) {
		raw := c.Query(key)
		if raw == "" {
			return nil, nil
		}
		value, err := time.ParseInLocation("2006-01-02", raw, time.Local)
		if err != nil {
			return nil, fmt.Errorf("%s 必须为 YYYY-MM-DD", key)
		}
		if end {
			value = value.AddDate(0, 0, 1)
		}
		return &value, nil
	}
	var err error
	if f.From, err = parseDate("from", false); err != nil {
		return f, err
	}
	if f.To, err = parseDate("to", true); err != nil {
		return f, err
	}
	parseNumber := func(key string) (uint64, error) {
		raw := c.Query(key)
		if raw == "" {
			return 0, nil
		}
		return strconv.ParseUint(raw, 10, 64)
	}
	if v, e := parseNumber("groupId"); e != nil {
		return f, fmt.Errorf("groupId 无效")
	} else {
		f.GroupID = int64(v)
	}
	if v, e := parseNumber("projectId"); e != nil {
		return f, fmt.Errorf("projectId 无效")
	} else {
		f.ProjectID = uint(v)
	}
	if v, e := parseNumber("userId"); e != nil {
		return f, fmt.Errorf("userId 无效")
	} else {
		f.UserID = uint(v)
	}
	return f, nil
}

// filteredCommits 过滤提交
func filteredCommits(db *gorm.DB, f filters) *gorm.DB {
	q := db.Model(&model.Commit{}).
		Joins("JOIN projects ON projects.id = commits.project_id").
		Joins("LEFT JOIN `groups` ON `groups`.id = projects.group_id").
		Where("commits.is_merge = ?", false)
	if f.From != nil {
		q = q.Where("commits.authored_at >= ?", *f.From)
	}
	if f.To != nil {
		q = q.Where("commits.authored_at < ?", *f.To)
	}
	if f.GroupID != 0 {
		q = q.Where("`groups`.root_git_lab_id = ?", f.GroupID)
	}
	if f.ProjectID != 0 {
		q = q.Where("commits.project_id = ?", f.ProjectID)
	}
	if f.UserID != 0 {
		q = q.Where("commits.user_id = ?", f.UserID)
	}
	return q
}

// pageParams 分页参数
func pageParams(c *gin.Context) (int, int) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("pageSize", "30"))
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 30
	}
	if size > 100 {
		size = 100
	}
	return page, size
}
