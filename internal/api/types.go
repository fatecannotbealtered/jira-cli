package api

// ===== Common =====

// PagedResponse contains pagination information.
type PagedResponse struct {
	StartAt    int  `json:"startAt"`
	MaxResults int  `json:"maxResults"`
	Total      int  `json:"total"`
	IsLast     bool `json:"isLast"`
}

// ===== Issue =====

type Issue struct {
	ID     string      `json:"id"`
	Key    string      `json:"key"`
	Self   string      `json:"self"`
	Fields IssueFields `json:"fields"`
}

type IssueFields struct {
	Summary     string       `json:"summary"`
	Description any          `json:"description"`
	Status      Status       `json:"status"`
	IssueType   IssueType    `json:"issuetype"`
	Priority    *Priority    `json:"priority"`
	Assignee    *User        `json:"assignee"`
	Reporter    *User        `json:"reporter"`
	Created     string       `json:"created"`
	Updated     string       `json:"updated"`
	Labels      []string     `json:"labels"`
	Components  []Component  `json:"components"`
	FixVersions []Version    `json:"fixVersions"`
	Parent      *IssueRef    `json:"parent"`
	Subtasks    []IssueRef   `json:"subtasks"`
	IssueLinks  []IssueLink  `json:"issuelinks"`
	Attachments []Attachment `json:"attachment"`
	Comment     CommentPage  `json:"comment"`
	Worklog     WorklogPage  `json:"worklog"`
	Watches     Watches      `json:"watches"`
	Votes       Votes        `json:"votes"`
}

type Status struct {
	ID             string         `json:"id"`
	Name           string         `json:"name"`
	StatusCategory StatusCategory `json:"statusCategory"`
}

type StatusCategory struct {
	Key       string `json:"key"`
	ColorName string `json:"colorName"`
}

type IssueType struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Subtask bool   `json:"subtask"`
}

type Priority struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type User struct {
	Name         string `json:"name"`
	DisplayName  string `json:"displayName"`
	EmailAddress string `json:"emailAddress"`
	Active       bool   `json:"active"`
}

type Component struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Version struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Released    bool   `json:"released"`
	ReleaseDate string `json:"releaseDate"`
	Description string `json:"description"`
}

type IssueRef struct {
	ID     string          `json:"id"`
	Key    string          `json:"key"`
	Fields *IssueRefFields `json:"fields,omitempty"`
}

// IssueRefFields carries the lightweight fields Jira embeds on a referenced
// issue inside issuelinks (summary + status). Present only when the parent
// payload expands them; nil otherwise.
type IssueRefFields struct {
	Summary string `json:"summary"`
	Status  Status `json:"status"`
}

type IssueLink struct {
	ID           string    `json:"id"`
	Type         LinkType  `json:"type"`
	InwardIssue  *IssueRef `json:"inwardIssue"`
	OutwardIssue *IssueRef `json:"outwardIssue"`
}

type LinkType struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Inward  string `json:"inward"`
	Outward string `json:"outward"`
}

type Attachment struct {
	ID       string `json:"id"`
	Filename string `json:"filename"`
	Size     int    `json:"size"`
	MimeType string `json:"mimeType"`
	Author   User   `json:"author"`
	Created  string `json:"created"`
	Content  string `json:"content"` // URL to download the raw bytes
}

type Comment struct {
	ID      string `json:"id"`
	Author  User   `json:"author"`
	Body    any    `json:"body"`
	Created string `json:"created"`
	Updated string `json:"updated"`
}

type CommentPage struct {
	Comments   []Comment `json:"comments"`
	Total      int       `json:"total"`
	StartAt    int       `json:"startAt"`
	MaxResults int       `json:"maxResults"`
}

type Worklog struct {
	ID               string `json:"id"`
	Author           User   `json:"author"`
	Comment          any    `json:"comment"`
	Started          string `json:"started"`
	TimeSpent        string `json:"timeSpent"`
	TimeSpentSeconds int    `json:"timeSpentSeconds"`
}

type WorklogPage struct {
	Worklogs   []Worklog `json:"worklogs"`
	Total      int       `json:"total"`
	StartAt    int       `json:"startAt"`
	MaxResults int       `json:"maxResults"`
}

type Watches struct {
	WatchCount int  `json:"watchCount"`
	IsWatching bool `json:"isWatching"`
}

type Votes struct {
	Votes    int  `json:"votes"`
	HasVoted bool `json:"hasVoted"`
}

type Transition struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	To   Status `json:"to"`
}

type RemoteLink struct {
	ID     int `json:"id"`
	Object struct {
		URL   string `json:"url"`
		Title string `json:"title"`
	} `json:"object"`
}

// ===== Search =====

type SearchResult struct {
	PagedResponse
	Issues []Issue `json:"issues"`
}

// ===== Sprint =====

type Sprint struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	State         string `json:"state"`
	StartDate     string `json:"startDate"`
	EndDate       string `json:"endDate"`
	CompleteDate  string `json:"completeDate"`
	Goal          string `json:"goal"`
	OriginBoardID int    `json:"originBoardId"`
}

type SprintPage struct {
	PagedResponse
	Values []Sprint `json:"values"`
}

// SprintReport is the flattened velocity/completion summary for one sprint,
// derived either from the Greenhopper sprintreport chart or computed from the
// sprint's issues when that endpoint is unavailable.
type SprintReport struct {
	SprintID           int     `json:"sprintId"`
	SprintName         string  `json:"sprintName"`
	State              string  `json:"state"`
	CommittedIssues    int     `json:"committedIssues"`
	CompletedIssues    int     `json:"completedIssues"`
	NotCompletedIssues int     `json:"notCompletedIssues"`
	PunctedIssues      int     `json:"removedIssues"`
	CommittedPoints    float64 `json:"committedPoints"`
	CompletedPoints    float64 `json:"completedPoints"`
	NotCompletedPoints float64 `json:"notCompletedPoints"`
	// Source reports how the figures were obtained: "greenhopper" (chart
	// endpoint) or "computed" (derived from sprint issues). Lets callers judge
	// confidence, since DC versions vary.
	Source string `json:"source"`
}

// greenhopperSprintReport mirrors the subset of the Greenhopper sprintreport
// chart payload jira-cli consumes. Field availability varies by DC version, so
// every nested value is optional and parsed defensively.
type greenhopperSprintReport struct {
	Contents struct {
		CompletedIssues                   []greenhopperReportIssue `json:"completedIssues"`
		IssuesNotCompletedInCurrentSprint []greenhopperReportIssue `json:"issuesNotCompletedInCurrentSprint"`
		PuntedIssues                      []greenhopperReportIssue `json:"puntedIssues"`
		CompletedIssuesEstimateSum        greenhopperEstimate      `json:"completedIssuesEstimateSum"`
		IssuesNotCompletedEstimateSum     greenhopperEstimate      `json:"issuesNotCompletedEstimateSum"`
		AllIssuesEstimateSum              greenhopperEstimate      `json:"allIssuesEstimateSum"`
	} `json:"contents"`
	Sprint struct {
		ID    int    `json:"id"`
		Name  string `json:"name"`
		State string `json:"state"`
	} `json:"sprint"`
}

type greenhopperReportIssue struct {
	Key               string `json:"key"`
	EstimateStatistic struct {
		StatFieldValue struct {
			Value float64 `json:"value"`
		} `json:"statFieldValue"`
	} `json:"estimateStatistic"`
}

type greenhopperEstimate struct {
	Value float64 `json:"value"`
	Text  string  `json:"text"`
}

// ===== Board =====

type Board struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Location struct {
		ProjectKey  string `json:"projectKey"`
		DisplayName string `json:"displayName"`
	} `json:"location"`
	Filter struct {
		ID string `json:"id"`
	} `json:"filter"`
}

type BoardPage struct {
	PagedResponse
	Values []Board `json:"values"`
}

// ===== Project =====

type Project struct {
	ID             string      `json:"id"`
	Key            string      `json:"key"`
	Name           string      `json:"name"`
	ProjectTypeKey string      `json:"projectTypeKey"`
	Lead           *User       `json:"lead"`
	Description    string      `json:"description"`
	URL            string      `json:"url"`
	IssueTypes     []IssueType `json:"issueTypes"`
	Style          string      `json:"style"`
}

type ProjectPage struct {
	PagedResponse
	Values []Project `json:"values"`
}

// ===== Filter =====

type Filter struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	Description      string `json:"description"`
	JQL              string `json:"jql"`
	Favourite        bool   `json:"favourite"`
	SharePermissions []struct {
		Type string `json:"type"`
	} `json:"sharePermissions"`
}

// ===== Myself =====

type Myself struct {
	Name         string `json:"name"`
	DisplayName  string `json:"displayName"`
	EmailAddress string `json:"emailAddress"`
	Active       bool   `json:"active"`
}

// ===== Request types =====

type CreateIssueRequest struct {
	Fields CreateIssueFields `json:"fields"`
}

type CreateIssueFields struct {
	Project     ProjectRef   `json:"project"`
	Summary     string       `json:"summary"`
	IssueType   IssueTypeRef `json:"issuetype"`
	Description any          `json:"description,omitempty"`
	Assignee    *NameRef     `json:"assignee,omitempty"`
	Priority    *NameRef     `json:"priority,omitempty"`
	Labels      []string     `json:"labels,omitempty"`
	Components  []NameRef    `json:"components,omitempty"`
	FixVersions []NameRef    `json:"fixVersions,omitempty"`
	Parent      *KeyRef      `json:"parent,omitempty"`
}

type ProjectRef struct {
	Key string `json:"key"`
}
type IssueTypeRef struct {
	Name string `json:"name"`
}
type NameRef struct {
	Name string `json:"name"`
}
type KeyRef struct {
	Key string `json:"key"`
}

type EditIssueRequest struct {
	Fields EditIssueFields `json:"fields"`
}

type EditIssueFields struct {
	Summary     string    `json:"summary,omitempty"`
	Description any       `json:"description,omitempty"`
	Assignee    *NameRef  `json:"assignee,omitempty"`
	Priority    *NameRef  `json:"priority,omitempty"`
	Labels      []string  `json:"labels,omitempty"`
	Components  []NameRef `json:"components,omitempty"`
	FixVersions []NameRef `json:"fixVersions,omitempty"`
}

type TransitionRequest struct {
	Transition struct {
		ID string `json:"id"`
	} `json:"transition"`
}

type IssueLinkRequest struct {
	Type         LinkTypeRef `json:"type"`
	InwardIssue  KeyRef      `json:"inwardIssue"`
	OutwardIssue KeyRef      `json:"outwardIssue"`
}

type LinkTypeRef struct {
	Name string `json:"name"`
}

type RemoteLinkRequest struct {
	Object RemoteLinkObject `json:"object"`
}

type RemoteLinkObject struct {
	URL   string `json:"url"`
	Title string `json:"title"`
}

type WorklogRequest struct {
	TimeSpentSeconds int    `json:"timeSpentSeconds"`
	Started          string `json:"started"`
	Comment          any    `json:"comment,omitempty"`
}

type CreateSprintRequest struct {
	Name          string `json:"name"`
	OriginBoardID int    `json:"originBoardId"`
	StartDate     string `json:"startDate,omitempty"`
	EndDate       string `json:"endDate,omitempty"`
	Goal          string `json:"goal,omitempty"`
}

type UpdateSprintRequest struct {
	Name      string `json:"name,omitempty"`
	Goal      string `json:"goal,omitempty"`
	StartDate string `json:"startDate,omitempty"`
	EndDate   string `json:"endDate,omitempty"`
	State     string `json:"state,omitempty"`
}

type MoveIssuesToSprintRequest struct {
	Issues []string `json:"issues"`
}

// RankRequest reorders issues relative to an anchor via the agile /issue/rank
// endpoint. Exactly one of RankAfterIssue / RankBeforeIssue is set.
type RankRequest struct {
	Issues            []string `json:"issues"`
	RankAfterIssue    string   `json:"rankAfterIssue,omitempty"`
	RankBeforeIssue   string   `json:"rankBeforeIssue,omitempty"`
	RankCustomFieldID int      `json:"rankCustomFieldId,omitempty"`
}

type CreateFilterRequest struct {
	Name        string `json:"name"`
	JQL         string `json:"jql"`
	Description string `json:"description,omitempty"`
}

type SearchOptions struct {
	StartAt    int
	MaxResults int
	Fields     []string
}

type IssueListOptions struct {
	Project  string
	Status   string
	Assignee string
	Type     string
	Label    string
	Priority string
	Limit    int
	OrderBy  string
}

// ===== Custom Fields =====

// FieldMeta describes the metadata for a Jira field.
type FieldMeta struct {
	ID          string       `json:"id"`
	Key         string       `json:"key"`
	Name        string       `json:"name"`
	Custom      bool         `json:"custom"`
	Navigable   bool         `json:"navigable"`
	Searchable  bool         `json:"searchable"`
	ClauseNames []string     `json:"clauseNames"`
	Schema      *FieldSchema `json:"schema,omitempty"`
}

// FieldSchema describes the type information for a field.
type FieldSchema struct {
	Type     string `json:"type"`
	Items    string `json:"items,omitempty"`
	Custom   string `json:"custom,omitempty"`
	CustomID int    `json:"customId,omitempty"`
	System   string `json:"system,omitempty"`
}
