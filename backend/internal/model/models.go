package model

import "time"

// Setting 系统设置
type Setting struct {
	ID            uint       `gorm:"primaryKey;comment:主键" json:"id"`                          // 主键
	GitLabURL     string     `gorm:"size:500;comment:GitLab 实例地址" json:"gitlabUrl"`            // GitLab URL
	GitLabToken   string     `gorm:"type:text;comment:GitLab 访问令牌" json:"-"`                   // GitLab Token
	RootGroupIDs  string     `gorm:"type:text;comment:根项目组 ID 列表，逗号/空格分隔" json:"rootGroupIds"` // 根组ID
	SyncCron      string     `gorm:"size:100;comment:同步 Cron 表达式" json:"syncCron"`             // 同步cron
	LastSuccessAt *time.Time `gorm:"comment:上次同步成功时间" json:"lastSuccessAt"`                    // 最后成功时间
	CreatedAt     time.Time  `gorm:"comment:创建时间" json:"createdAt"`                            // 创建时间
	UpdatedAt     time.Time  `gorm:"comment:更新时间" json:"updatedAt"`                            // 更新时间
}

// Group GitLab组
type Group struct {
	ID           uint      `gorm:"primaryKey;comment:主键" json:"id"`                    // 主键
	GitLabID     int64     `gorm:"uniqueIndex;comment:GitLab 组 ID" json:"gitlabId"`    // GitLab组ID
	RootGitLabID int64     `gorm:"index;comment:所属根项目组 GitLab ID" json:"rootGitlabId"` // 根组ID
	Name         string    `gorm:"size:255;comment:组名" json:"name"`                    // 组名
	FullPath     string    `gorm:"size:500;index;comment:组完整路径" json:"fullPath"`       // 完整路径
	WebURL       string    `gorm:"size:1000;comment:GitLab 组页面链接" json:"webUrl"`       // Web URL
	CreatedAt    time.Time `gorm:"comment:创建时间" json:"createdAt"`                      // 创建时间
	UpdatedAt    time.Time `gorm:"comment:更新时间" json:"updatedAt"`                      // 更新时间
}

// Project GitLab项目
type Project struct {
	ID            uint      `gorm:"primaryKey;comment:主键" json:"id"`                              // 主键
	GitLabID      int64     `gorm:"uniqueIndex;comment:GitLab 项目 ID" json:"gitlabId"`             // GitLab项目ID
	GroupID       *uint     `gorm:"index;comment:所属组 ID，可为空" json:"groupId"`                      // 组ID
	Name          string    `gorm:"size:255;comment:项目名" json:"name"`                             // 项目名
	PathWithNS    string    `gorm:"size:500;index;comment:项目带命名空间的完整路径" json:"pathWithNamespace"` // 路径
	WebURL        string    `gorm:"size:1000;comment:GitLab 项目页面链接" json:"webUrl"`                // Web URL
	DefaultBranch string    `gorm:"size:255;comment:默认分支" json:"defaultBranch"`                   // 默认分支
	Archived      bool      `gorm:"comment:是否已归档" json:"archived"`                                // 是否归档
	CreatedAt     time.Time `gorm:"comment:创建时间" json:"createdAt"`                                // 创建时间
	UpdatedAt     time.Time `gorm:"comment:更新时间" json:"updatedAt"`                                // 更新时间
	Group         *Group    `gorm:"comment:关联组" json:"group,omitempty"`                           // 组
}

// User GitLab用户
type User struct {
	ID        uint      `gorm:"primaryKey;comment:主键" json:"id"`                  // 主键
	Name      string    `gorm:"size:255;index;comment:用户姓名" json:"name"`          // 用户名
	Email     string    `gorm:"size:320;uniqueIndex;comment:邮箱（小写）" json:"email"` // 邮箱
	Username  string    `gorm:"size:255;index;comment:用户名" json:"username"`       // 用户名
	AvatarURL string    `gorm:"size:1000;comment:头像链接" json:"avatarUrl"`          // 头像URL
	IsBot     bool      `gorm:"comment:是否为机器人账号" json:"isBot"`                    // 是否为机器人
	CreatedAt time.Time `gorm:"comment:创建时间" json:"createdAt"`                    // 创建时间
	UpdatedAt time.Time `gorm:"comment:更新时间" json:"updatedAt"`                    // 更新时间
}

// UserAlias 用户别名
type UserAlias struct {
	ID        uint      `gorm:"primaryKey;comment:主键" json:"id"`                    // 主键
	UserID    uint      `gorm:"index;comment:所属用户 ID" json:"userId"`                // 用户ID
	Email     string    `gorm:"size:320;uniqueIndex;comment:邮箱别名（小写）" json:"email"` // 邮箱
	CreatedAt time.Time `gorm:"comment:创建时间" json:"createdAt"`                      // 创建时间
	User      User      `gorm:"comment:关联用户" json:"user"`                           // 用户
}

// Commit GitLab提交
type Commit struct {
	ID           uint      `gorm:"primaryKey;comment:主键" json:"id"`                                                                  // 主键
	ProjectID    uint      `gorm:"not null;uniqueIndex:idx_project_sha;index:idx_project_authored;comment:所属项目 ID" json:"projectId"` // 项目ID
	UserID       *uint     `gorm:"index:idx_user_authored;comment:贡献者 ID，可为空" json:"userId"`                                         // 用户ID
	SHA          string    `gorm:"size:64;not null;uniqueIndex:idx_project_sha;comment:提交完整 SHA" json:"sha"`                         // 提交SHA
	ShortSHA     string    `gorm:"size:16;comment:提交短 SHA" json:"shortSha"`                                                          // 短SHA
	Title        string    `gorm:"size:500;comment:提交标题" json:"title"`                                                               // 提交标题
	Message      string    `gorm:"type:text;comment:提交完整信息" json:"message"`                                                          // 提交消息
	AuthorName   string    `gorm:"size:255;comment:作者名（GitLab 原始值）" json:"authorName"`                                               // 作者名
	AuthorEmail  string    `gorm:"size:320;index;comment:作者邮箱（小写）" json:"authorEmail"`                                               // 作者邮箱
	AuthoredAt   time.Time `gorm:"index;comment:作者提交时间" json:"authoredAt"`                                                           // 作者时间
	CommittedAt  time.Time `gorm:"comment:Git 提交时间" json:"committedAt"`                                                              // 提交时间
	Additions    int       `gorm:"comment:新增行数" json:"additions"`                                                                    // 增加行数
	Deletions    int       `gorm:"comment:删除行数" json:"deletions"`                                                                    // 删除行数
	TotalChanges int       `gorm:"comment:变更总行数" json:"totalChanges"`                                                                // 总变更行数
	IsMerge      bool      `gorm:"index;comment:是否为合并提交" json:"isMerge"`                                                             // 是否为合并提交
	WebURL       string    `gorm:"size:1000;comment:GitLab 提交链接" json:"webUrl"`                                                      // Web URL
	CreatedAt    time.Time `gorm:"comment:创建时间" json:"createdAt"`                                                                    // 创建时间
	UpdatedAt    time.Time `gorm:"comment:更新时间" json:"updatedAt"`                                                                    // 更新时间
	Project      Project   `gorm:"comment:关联项目" json:"project,omitempty"`                                                            // 项目
	User         *User     `gorm:"comment:关联贡献者" json:"user,omitempty"`                                                              // 用户
}

// SyncRun 同步运行记录
type SyncRun struct {
	ID              uint       `gorm:"primaryKey;comment:主键" json:"id"`                                  // 主键
	Mode            string     `gorm:"size:20;comment:同步模式（full/incremental）" json:"mode"`               // 同步模式
	Status          string     `gorm:"size:20;index;comment:运行状态（running/success/failed）" json:"status"` // 状态
	StartedAt       time.Time  `gorm:"index;comment:开始时间" json:"startedAt"`                              // 开始时间
	FinishedAt      *time.Time `gorm:"comment:结束时间" json:"finishedAt"`                                   // 结束时间
	ProjectsScanned int        `gorm:"comment:本次扫描的项目数" json:"projectsScanned"`                          // 扫描的项目数
	CommitsSynced   int        `gorm:"comment:本次同步的提交数" json:"commitsSynced"`                            // 同步的提交数
	ErrorMessage    string     `gorm:"type:text;comment:错误信息" json:"errorMessage"`                       // 错误信息
}
