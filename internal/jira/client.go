package jira

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client is a Jira Cloud REST API client.
type Client struct {
	baseURL    string
	username   string
	token      string
	httpClient *http.Client
}

// NewClient creates a Jira Cloud API client using email + API token basic auth.
func NewClient(baseURL, username, token string) (*Client, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return nil, errors.New("jira base URL is required")
	}
	if username == "" {
		return nil, errors.New("jira username is required")
	}
	if token == "" {
		return nil, errors.New("jira API token is required")
	}
	return &Client{
		baseURL:  baseURL,
		username: username,
		token:    token,
		httpClient: &http.Client{
			Timeout: 45 * time.Second,
		},
	}, nil
}

// Myself returns the authenticated Jira user.
func (c *Client) Myself(ctx context.Context) (User, error) {
	var raw struct {
		AccountID    string `json:"accountId"`
		DisplayName  string `json:"displayName"`
		EmailAddress string `json:"emailAddress"`
	}
	if err := c.do(ctx, http.MethodGet, "/rest/api/3/myself", nil, nil, &raw); err != nil {
		return User{}, err
	}
	return User{AccountID: raw.AccountID, DisplayName: raw.DisplayName, Email: raw.EmailAddress}, nil
}

// SearchProjects returns Jira projects matching query.
func (c *Client) SearchProjects(ctx context.Context, query string) ([]Project, error) {
	params := url.Values{"maxResults": {"50"}}
	if query != "" {
		params.Set("query", query)
	}
	var raw struct {
		Values []struct {
			ID   string `json:"id"`
			Key  string `json:"key"`
			Name string `json:"name"`
		} `json:"values"`
	}
	if err := c.do(ctx, http.MethodGet, "/rest/api/3/project/search", params, nil, &raw); err != nil {
		return nil, err
	}
	projects := make([]Project, 0, len(raw.Values))
	for _, p := range raw.Values {
		projects = append(projects, Project{ID: p.ID, Key: p.Key, Name: p.Name})
	}
	return projects, nil
}

// Project returns a Jira project with its issue types.
func (c *Client) Project(ctx context.Context, projectKeyOrID string) (Project, error) {
	if strings.TrimSpace(projectKeyOrID) == "" {
		return Project{}, errors.New("project key is required")
	}
	path := fmt.Sprintf("/rest/api/3/project/%s", url.PathEscape(projectKeyOrID))
	params := url.Values{"expand": {"issueTypes"}}
	var raw struct {
		ID         string          `json:"id"`
		Key        string          `json:"key"`
		Name       string          `json:"name"`
		IssueTypes []issueTypeJSON `json:"issueTypes"`
	}
	if err := c.do(ctx, http.MethodGet, path, params, nil, &raw); err != nil {
		return Project{}, err
	}
	project := Project{ID: raw.ID, Key: raw.Key, Name: raw.Name, IssueTypes: make([]IssueType, 0, len(raw.IssueTypes))}
	for _, issueType := range raw.IssueTypes {
		project.IssueTypes = append(project.IssueTypes, issueType.issueType())
	}
	return project, nil
}

// ProjectIssueTypes returns issue types available to a project.
func (c *Client) ProjectIssueTypes(ctx context.Context, projectID string) ([]IssueType, error) {
	if strings.TrimSpace(projectID) == "" {
		return nil, errors.New("project id is required")
	}
	params := url.Values{"projectId": {projectID}}
	var raw []issueTypeJSON
	if err := c.do(ctx, http.MethodGet, "/rest/api/3/issuetype/project", params, nil, &raw); err != nil {
		return nil, err
	}
	issueTypes := make([]IssueType, 0, len(raw))
	for _, issueType := range raw {
		issueTypes = append(issueTypes, issueType.issueType())
	}
	return issueTypes, nil
}

// SearchBoards returns agile boards for a project key or id.
func (c *Client) SearchBoards(ctx context.Context, projectKeyOrID string) ([]Board, error) {
	params := url.Values{"maxResults": {"50"}}
	if projectKeyOrID != "" {
		params.Set("projectKeyOrId", projectKeyOrID)
	}
	var raw struct {
		Values []struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
			Type string `json:"type"`
		} `json:"values"`
	}
	if err := c.do(ctx, http.MethodGet, "/rest/agile/1.0/board", params, nil, &raw); err != nil {
		return nil, err
	}
	boards := make([]Board, 0, len(raw.Values))
	for _, b := range raw.Values {
		boards = append(boards, Board{ID: b.ID, Name: b.Name, Type: b.Type})
	}
	return boards, nil
}

// ActiveSprint returns the active sprint for a Scrum board.
func (c *Client) ActiveSprint(ctx context.Context, boardID int) (Sprint, error) {
	params := url.Values{"state": {"active"}, "maxResults": {"10"}}
	path := fmt.Sprintf("/rest/agile/1.0/board/%d/sprint", boardID)
	var raw struct {
		Values []sprintJSON `json:"values"`
	}
	if err := c.do(ctx, http.MethodGet, path, params, nil, &raw); err != nil {
		return Sprint{}, err
	}
	if len(raw.Values) == 0 {
		return Sprint{}, errors.New("no active sprint found for configured board")
	}
	return parseSprint(raw.Values[0]), nil
}

// BoardConfiguration returns agile board settings such as the estimation field.
func (c *Client) BoardConfiguration(ctx context.Context, boardID int) (BoardConfiguration, error) {
	path := fmt.Sprintf("/rest/agile/1.0/board/%d/configuration", boardID)
	var raw struct {
		Estimation struct {
			Field struct {
				FieldID     string `json:"fieldId"`
				DisplayName string `json:"displayName"`
			} `json:"field"`
		} `json:"estimation"`
	}
	if err := c.do(ctx, http.MethodGet, path, nil, nil, &raw); err != nil {
		return BoardConfiguration{}, err
	}
	return BoardConfiguration{EstimationFieldID: raw.Estimation.Field.FieldID, EstimationFieldName: raw.Estimation.Field.DisplayName}, nil
}

// Fields returns Jira fields visible to the authenticated user.
func (c *Client) Fields(ctx context.Context) ([]Field, error) {
	var raw []struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Custom bool   `json:"custom"`
		Schema struct {
			Type string `json:"type"`
		} `json:"schema"`
	}
	if err := c.do(ctx, http.MethodGet, "/rest/api/3/field", nil, nil, &raw); err != nil {
		return nil, err
	}
	fields := make([]Field, 0, len(raw))
	for _, field := range raw {
		fields = append(fields, Field{ID: field.ID, Name: field.Name, Custom: field.Custom, SchemaType: field.Schema.Type})
	}
	return fields, nil
}

// SearchMySprintIssues returns issues assigned to the current user in a sprint.
func (c *Client) SearchMySprintIssues(ctx context.Context, projectKey string, sprintID int, storyPointsFieldID string) ([]Issue, error) {
	jql := fmt.Sprintf(`project = %q AND sprint = %d AND assignee = currentUser() ORDER BY Rank ASC`, projectKey, sprintID)
	fields := []string{"summary", "status", "assignee", "issuetype", "updated"}
	if storyPointsFieldID != "" {
		fields = append(fields, storyPointsFieldID)
	}
	return c.searchIssues(ctx, jql, fields, storyPointsFieldID)
}

// SearchUpdatedIssues returns sprint issues updated since the given time.
func (c *Client) SearchUpdatedIssues(ctx context.Context, projectKey string, sprintID int, since time.Time, storyPointsFieldID string) ([]Issue, error) {
	jql := fmt.Sprintf(`project = %q AND sprint = %d AND updated >= %q ORDER BY updated DESC`, projectKey, sprintID, since.Format("2006-01-02 15:04"))
	fields := []string{"summary", "status", "assignee", "issuetype", "updated"}
	if storyPointsFieldID != "" {
		fields = append(fields, storyPointsFieldID)
	}
	return c.searchIssues(ctx, jql, fields, storyPointsFieldID)
}

// CreateTask creates a Jira Task. Add it to a sprint with AddIssuesToSprint afterwards.
func (c *Client) CreateTask(ctx context.Context, input CreateTaskInput) (Issue, error) {
	fields := map[string]any{
		"project":   map[string]string{"key": input.ProjectKey},
		"issuetype": map[string]string{"id": input.IssueTypeID},
		"summary":   input.Summary,
	}
	if input.AssigneeID != "" {
		fields["assignee"] = map[string]string{"accountId": input.AssigneeID}
	}
	if input.Description != "" {
		fields["description"] = textToADF(input.Description)
	}
	if input.StoryPoints != nil && input.StoryPointsID != "" {
		fields[input.StoryPointsID] = *input.StoryPoints
	}
	maps.Copy(fields, input.AdditionalField)
	var raw struct {
		ID  string `json:"id"`
		Key string `json:"key"`
	}
	if err := c.do(ctx, http.MethodPost, "/rest/api/3/issue", nil, map[string]any{"fields": fields}, &raw); err != nil {
		return Issue{}, err
	}
	return Issue{ID: raw.ID, Key: raw.Key, Summary: input.Summary, StoryPoints: input.StoryPoints}, nil
}

// AddIssuesToSprint adds issues to a Jira sprint.
func (c *Client) AddIssuesToSprint(ctx context.Context, sprintID int, issueKeys []string) error {
	path := fmt.Sprintf("/rest/agile/1.0/sprint/%d/issue", sprintID)
	return c.do(ctx, http.MethodPost, path, nil, map[string]any{"issues": issueKeys}, nil)
}

// AssignIssue assigns an issue to a Jira Cloud account.
func (c *Client) AssignIssue(ctx context.Context, issueKey string, accountID string) error {
	if strings.TrimSpace(accountID) == "" {
		return errors.New("jira account id is required to assign issue")
	}
	path := fmt.Sprintf("/rest/api/3/issue/%s/assignee", url.PathEscape(issueKey))
	return c.do(ctx, http.MethodPut, path, nil, map[string]string{"accountId": accountID}, nil)
}

// Transitions returns all valid workflow transitions for an issue.
func (c *Client) Transitions(ctx context.Context, issueKey string) ([]Transition, error) {
	path := fmt.Sprintf("/rest/api/3/issue/%s/transitions", url.PathEscape(issueKey))
	var raw struct {
		Transitions []struct {
			ID   string     `json:"id"`
			Name string     `json:"name"`
			To   statusJSON `json:"to"`
		} `json:"transitions"`
	}
	if err := c.do(ctx, http.MethodGet, path, nil, nil, &raw); err != nil {
		return nil, err
	}
	transitions := make([]Transition, 0, len(raw.Transitions))
	for _, t := range raw.Transitions {
		transitions = append(transitions, Transition{ID: t.ID, Name: t.Name, ToStatus: parseStatus(t.To)})
	}
	return transitions, nil
}

// TransitionIssue applies a Jira workflow transition.
func (c *Client) TransitionIssue(ctx context.Context, issueKey, transitionID string) error {
	path := fmt.Sprintf("/rest/api/3/issue/%s/transitions", url.PathEscape(issueKey))
	payload := map[string]any{"transition": map[string]string{"id": transitionID}}
	return c.do(ctx, http.MethodPost, path, nil, payload, nil)
}

// UpdateStoryPoints sets or clears the story points custom field for an issue.
func (c *Client) UpdateStoryPoints(ctx context.Context, issueKey, storyPointsFieldID string, points *float64) error {
	if storyPointsFieldID == "" {
		return errors.New("story points field id is not configured")
	}
	path := fmt.Sprintf("/rest/api/3/issue/%s", url.PathEscape(issueKey))
	fields := map[string]any{storyPointsFieldID: nil}
	if points != nil {
		fields[storyPointsFieldID] = *points
	}
	return c.do(ctx, http.MethodPut, path, nil, map[string]any{"fields": fields}, nil)
}

func (c *Client) searchIssues(ctx context.Context, jql string, fields []string, storyPointsFieldID string) ([]Issue, error) {
	issues := []Issue{}
	nextToken := ""
	for {
		payload := map[string]any{"jql": jql, "maxResults": 50, "fields": fields}
		if nextToken != "" {
			payload["nextPageToken"] = nextToken
		}
		var raw struct {
			Issues        []issueJSON `json:"issues"`
			NextPageToken string      `json:"nextPageToken"`
			IsLast        bool        `json:"isLast"`
		}
		if err := c.do(ctx, http.MethodPost, "/rest/api/3/search/jql", nil, payload, &raw); err != nil {
			return nil, err
		}
		for _, issue := range raw.Issues {
			issues = append(issues, parseIssue(issue, storyPointsFieldID))
		}
		if raw.IsLast || raw.NextPageToken == "" {
			break
		}
		nextToken = raw.NextPageToken
	}
	return issues, nil
}

func (c *Client) do(ctx context.Context, method, path string, query url.Values, body any, out any) error {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
		reader = bytes.NewReader(data)
	}
	reqURL := c.baseURL + path
	if len(query) > 0 {
		reqURL += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, reqURL, reader)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.SetBasicAuth(c.username, c.token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("jira request failed: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return jiraHTTPError(resp.StatusCode, data)
	}
	if out == nil || len(data) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

func jiraHTTPError(status int, data []byte) error {
	var raw struct {
		ErrorMessages []string          `json:"errorMessages"`
		Errors        map[string]string `json:"errors"`
	}
	if err := json.Unmarshal(data, &raw); err == nil {
		if len(raw.ErrorMessages) > 0 {
			return fmt.Errorf("jira API error %d: %s", status, strings.Join(raw.ErrorMessages, "; "))
		}
		if len(raw.Errors) > 0 {
			parts := make([]string, 0, len(raw.Errors))
			for field, message := range raw.Errors {
				parts = append(parts, field+": "+message)
			}
			return fmt.Errorf("jira API error %d: %s", status, strings.Join(parts, "; "))
		}
	}
	return fmt.Errorf("jira API error %d: %s", status, strings.TrimSpace(string(data)))
}

func textToADF(text string) map[string]any {
	lines := strings.SplitSeq(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	var content []any
	for line := range lines {
		paragraph := map[string]any{"type": "paragraph"}
		if line != "" {
			paragraph["content"] = []any{map[string]any{"type": "text", "text": line}}
		}
		content = append(content, paragraph)
	}
	return map[string]any{"type": "doc", "version": 1, "content": content}
}
