package gitlab

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Client GitLab API 客户端
type Client struct {
	baseURL, token string
	http           *http.Client
}

// Group GitLab 组信息
type Group struct {
	ID          int64     `json:"id"`
	ParentID    int64     `json:"parent_id"`
	Name        string    `json:"name"`
	FullPath    string    `json:"full_path"`
	WebURL      string    `json:"web_url"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

// Namespace GitLab 命名空间信息
type Namespace struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	FullPath string `json:"full_path"`
}

// Project GitLab 项目信息
type Project struct {
	ID                int64             `json:"id"`
	Name              string            `json:"name"`
	PathWithNamespace string            `json:"path_with_namespace"`
	WebURL            string            `json:"web_url"`
	DefaultBranch     string            `json:"default_branch"`
	Archived          bool              `json:"archived"`
	Namespace         Namespace         `json:"namespace"`
	Description       string            `json:"description"`
	CreatedAt         time.Time         `json:"created_at"`
	Statistics        ProjectStatistics `json:"statistics"`
}

// Commit GitLab 提交信息
type Commit struct {
	ID            string    `json:"id"`
	ShortID       string    `json:"short_id"`
	Title         string    `json:"title"`
	Message       string    `json:"message"`
	AuthorName    string    `json:"author_name"`
	AuthorEmail   string    `json:"author_email"`
	AuthoredDate  time.Time `json:"authored_date"`
	CommittedDate time.Time `json:"committed_date"`
	ParentIDs     []string  `json:"parent_ids"`
	WebURL        string    `json:"web_url"`
	Stats         struct {
		Additions int `json:"additions"`
		Deletions int `json:"deletions"`
		Total     int `json:"total"`
	} `json:"stats"`
}

// Member GitLab 成员信息
type Member struct {
	ID           int64      `json:"id"`
	Name         string     `json:"name"`
	Username     string     `json:"username"`
	AccessLevel  int        `json:"access_level"`
	CreatedAt    *time.Time `json:"created_at"`
	ExpiresAt    *time.Time `json:"-"`
	ExpiresAtRaw *string    `json:"expires_at"`
}

// ProjectStatistics 项目统计信息
type ProjectStatistics struct {
	CommitCount           int64 `json:"commit_count"`
	StorageSize           int64 `json:"storage_size"`
	RepositorySize        int64 `json:"repository_size"`
	WikiSize              int64 `json:"wiki_size"`
	LFSObjectsSize        int64 `json:"lfs_objects_size"`
	JobArtifactsSize      int64 `json:"job_artifacts_size"`
	PipelineArtifactsSize int64 `json:"pipeline_artifacts_size"`
	PackagesSize          int64 `json:"packages_size"`
	SnippetsSize          int64 `json:"snippets_size"`
	UploadsSize           int64 `json:"uploads_size"`
	ContainerRegistrySize int64 `json:"container_registry_size"`
}

// Branch GitLab 分支信息
type Branch struct {
	Name      string `json:"name"`
	Commit    Commit `json:"commit"`
	Protected bool   `json:"protected"`
	Default   bool   `json:"default"`
	Developed bool   `json:"developers_can_push"`
	Merged    bool   `json:"can_push"`
	WebURL    string `json:"web_url"`
}

// New 创建 GitLab API 客户端
func New(baseURL, token string) (*Client, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" || token == "" {
		return nil, fmt.Errorf("GitLab 地址和访问令牌不能为空")
	}
	if _, err := url.ParseRequestURI(baseURL); err != nil {
		return nil, fmt.Errorf("GitLab 地址无效: %w", err)
	}
	return &Client{baseURL: baseURL + "/api/v4", token: token, http: &http.Client{Timeout: 45 * time.Second}}, nil
}

// Test 测试 GitLab API 连接
func (c *Client) Test(ctx context.Context) error {
	var profile map[string]any
	return c.get(ctx, "/user", nil, &profile)
}

// GetGroup 获取 GitLab 组信息
func (c *Client) GetGroup(ctx context.Context, id int64) (Group, error) {
	var group Group
	err := c.get(ctx, "/groups/"+strconv.FormatInt(id, 10), nil, &group)
	return group, err
}

// ListTopLevelGroups 获取所有顶级组
func (c *Client) ListTopLevelGroups(ctx context.Context) ([]Group, error) {
	var result []Group
	q := url.Values{"all_available": {"true"}, "top_level_only": {"true"}, "per_page": {"100"}, "order_by": {"id"}, "sort": {"asc"}}
	err := c.pages(ctx, "/groups", q, func(body []byte) error {
		var page []Group
		if err := json.Unmarshal(body, &page); err != nil {
			return err
		}
		result = append(result, page...)
		return nil
	})
	return result, err
}

// ListProjects 获取组下的所有项目
func (c *Client) ListProjects(ctx context.Context, groupID int64) ([]Project, error) {
	var result []Project
	path := "/groups/" + strconv.FormatInt(groupID, 10) + "/projects"
	q := url.Values{"include_subgroups": {"true"}, "per_page": {"100"}, "archived": {"false"}, "order_by": {"id"}, "sort": {"asc"}, "statistics": {"true"}}
	err := c.pages(ctx, path, q, func(body []byte) error {
		var page []Project
		if err := json.Unmarshal(body, &page); err != nil {
			return err
		}
		result = append(result, page...)
		return nil
	})
	return result, err
}

// ListSubGroups 获取指定 Group 下的所有直接子 Group
func (c *Client) ListSubGroups(ctx context.Context, groupID int64) ([]Group, error) {
	var result []Group

	path := fmt.Sprintf("/groups/%d/subgroups", groupID)

	query := url.Values{}
	query.Set("per_page", "100")
	query.Set("order_by", "id")
	query.Set("sort", "asc")

	err := c.pages(ctx, path, query, func(body []byte) error {
		var groups []Group
		if err := json.Unmarshal(body, &groups); err != nil {
			return fmt.Errorf("解析子组数据失败: %w", err)
		}

		result = append(result, groups...)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("获取 Group %d 子组失败: %w", groupID, err)
	}

	return result, nil
}

// ListCommits 获取项目提交记录
func (c *Client) ListCommits(ctx context.Context, projectID int64, since *time.Time) ([]Commit, error) {
	var result []Commit
	path := "/projects/" + strconv.FormatInt(projectID, 10) + "/repository/commits"
	q := url.Values{"with_stats": {"true"}, "per_page": {"100"}, "order": {"topo"}}
	if since != nil {
		q.Set("since", since.UTC().Format(time.RFC3339))
	}
	err := c.pages(ctx, path, q, func(body []byte) error {
		var page []Commit
		if err := json.Unmarshal(body, &page); err != nil {
			return err
		}
		result = append(result, page...)
		return nil
	})
	return result, err
}

// ListBranchCommits 获取指定分支的提交记录
func (c *Client) ListBranchCommits(
	ctx context.Context,
	projectID int64,
	branchName string,
	since *time.Time,
) ([]Commit, error) {
	var result []Commit

	path := "/projects/" + strconv.FormatInt(projectID, 10) +
		"/repository/commits"

	q := url.Values{
		"with_stats": {"true"},
		"per_page":   {"100"},
		"order":      {"topo"},
		"ref_name":   {branchName},
	}

	if since != nil {
		q.Set("since", since.UTC().Format(time.RFC3339))
	}

	err := c.pages(ctx, path, q, func(body []byte) error {
		var page []Commit
		if err := json.Unmarshal(body, &page); err != nil {
			return err
		}

		result = append(result, page...)
		return nil
	})

	return result, err
}

// ListAllBranchCommits 获取项目所有分支的提交记录
func (c *Client) ListAllBranchCommits(
	ctx context.Context,
	projectID int64,
	since *time.Time,
) ([]Commit, error) {
	// 获取所有分支
	branches, err := c.ListBranches(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("获取项目分支失败: %w", err)
	}

	// 按 Commit SHA 去重
	commitMap := make(map[string]Commit)

	for _, branch := range branches {
		commits, err := c.ListBranchCommits(ctx, projectID, branch.Name, since)
		if err != nil {
			return nil, fmt.Errorf(
				"获取分支 %s 提交记录失败: %w",
				branch.Name,
				err,
			)
		}

		for _, commit := range commits {
			if _, exists := commitMap[commit.ID]; !exists {
				commitMap[commit.ID] = commit
			}
		}
	}

	result := make([]Commit, 0, len(commitMap))
	for _, commit := range commitMap {
		result = append(result, commit)
	}

	// 按提交时间倒序排列
	sort.Slice(result, func(i, j int) bool {
		return result[i].CommittedDate.After(result[j].CommittedDate)
	})

	return result, nil
}

// ListBranches 获取项目所有分支
func (c *Client) ListBranches(ctx context.Context, projectID int64) ([]Branch, error) {
	var result []Branch

	path := "/projects/" + strconv.FormatInt(projectID, 10) + "/repository/branches"
	q := url.Values{
		"per_page": {"100"},
	}

	err := c.pages(ctx, path, q, func(body []byte) error {
		var page []Branch
		if err := json.Unmarshal(body, &page); err != nil {
			return err
		}

		result = append(result, page...)
		return nil
	})

	return result, err
}

// pages 分页获取数据
func (c *Client) pages(ctx context.Context, path string, q url.Values, consume func([]byte) error) error {
	for page := 1; ; page++ {
		query := url.Values{}
		for key, values := range q {
			query[key] = append([]string(nil), values...)
		}
		query.Set("page", strconv.Itoa(page))
		body, header, err := c.raw(ctx, path, query)
		if err != nil {
			return err
		}
		if err := consume(body); err != nil {
			return err
		}
		if header.Get("X-Next-Page") == "" {
			return nil
		}
	}
}

// get 获取 GitLab API 数据
func (c *Client) get(ctx context.Context, path string, q url.Values, target any) error {
	body, _, err := c.raw(ctx, path, q)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, target)
}

// raw 发送 GitLab API 请求
func (c *Client) raw(ctx context.Context, path string, q url.Values) ([]byte, http.Header, error) {
	endpoint := c.baseURL + path
	if len(q) > 0 {
		endpoint += "?" + q.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("PRIVATE-TOKEN", c.token)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("请求 GitLab 失败: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 20<<20))
	if err != nil {
		return nil, nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, nil, fmt.Errorf("GitLab API 返回 %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	return body, resp.Header, nil
}

// ListMembers 获取指定 Group 或 Project 的直接成员
// targetType: "groups" 或 "projects"
func (c *Client) ListMembers(
	ctx context.Context,
	targetType string,
	targetID int64,
) ([]Member, error) {
	return c.listMembers(ctx, targetType, targetID, false)
}

// ListAllMembers 获取指定 Group 或 Project 的所有成员（包括继承的）
// targetType: "groups" 或 "projects"
func (c *Client) ListAllMembers(
	ctx context.Context,
	targetType string,
	targetID int64,
) ([]Member, error) {
	return c.listMembers(ctx, targetType, targetID, true)
}

// listMembers 内部方法，用于获取成员列表
func (c *Client) listMembers(
	ctx context.Context,
	targetType string,
	targetID int64,
	all bool,
) ([]Member, error) {
	if targetType != "groups" && targetType != "projects" {
		return nil, fmt.Errorf("不支持的成员目标类型: %s", targetType)
	}

	endpoint := fmt.Sprintf("/%s/%d/members", targetType, targetID)
	if all {
		endpoint += "/all"
	}

	query := url.Values{}
	query.Set("per_page", "100")

	var result []Member

	err := c.pages(ctx, endpoint, query, func(body []byte) error {
		var members []Member
		if err := json.Unmarshal(body, &members); err != nil {
			return fmt.Errorf("解析成员数据失败: %w", err)
		}

		result = append(result, members...)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("获取 %s %d 成员失败: %w", targetType, targetID, err)
	}

	for i := range result {
		if result[i].ExpiresAtRaw == nil || *result[i].ExpiresAtRaw == "" {
			continue
		}

		expiresAt, err := time.Parse("2006-01-02", *result[i].ExpiresAtRaw)
		if err != nil {
			return nil, fmt.Errorf(
				"解析成员过期时间失败，userID=%d, expiresAt=%s: %w",
				result[i].ID,
				*result[i].ExpiresAtRaw,
				err,
			)
		}

		result[i].ExpiresAt = &expiresAt
	}

	return result, nil
}

// ListInvitedGroups 获取当前 Group 直接邀请进来的 Group。
// 这些 Group 是当前 Group 的共享权限来源。
func (c *Client) ListInvitedGroups(
	ctx context.Context,
	groupID int64,
) ([]Group, error) {
	var result []Group

	path := fmt.Sprintf(
		"/groups/%d/invited_groups",
		groupID,
	)

	query := url.Values{}
	query.Set("per_page", "100")
	query.Set("relation", "direct")
	query.Set("order_by", "id")
	query.Set("sort", "asc")

	err := c.pages(ctx, path, query, func(body []byte) error {
		var groups []Group

		if err := json.Unmarshal(body, &groups); err != nil {
			return fmt.Errorf(
				"解析邀请群组数据失败: %w",
				err,
			)
		}

		result = append(result, groups...)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf(
			"获取 Group %d 的邀请群组失败: %w",
			groupID,
			err,
		)
	}

	return result, nil
}

// GetProject 获取项目详情及统计信息
func (c *Client) GetProject(ctx context.Context, projectID int64) (*Project, error) {
	var project Project

	path := fmt.Sprintf("/projects/%d?statistics=true", projectID)

	if err := c.get(ctx, path, nil, &project); err != nil {
		return nil, err
	}

	return &project, nil
}
