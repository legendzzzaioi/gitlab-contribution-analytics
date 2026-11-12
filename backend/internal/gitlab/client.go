package gitlab

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
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
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	FullPath string `json:"full_path"`
	WebURL   string `json:"web_url"`
}

// Namespace GitLab 命名空间信息
type Namespace struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	FullPath string `json:"full_path"`
}

// Project GitLab 项目信息
type Project struct {
	ID                int64     `json:"id"`
	Name              string    `json:"name"`
	PathWithNamespace string    `json:"path_with_namespace"`
	WebURL            string    `json:"web_url"`
	DefaultBranch     string    `json:"default_branch"`
	Archived          bool      `json:"archived"`
	Namespace         Namespace `json:"namespace"`
}
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
	q := url.Values{"include_subgroups": {"true"}, "per_page": {"100"}, "archived": {"false"}, "order_by": {"id"}, "sort": {"asc"}}
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
