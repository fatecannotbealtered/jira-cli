package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/fatecannotbealtered/jira-cli/internal/output"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type referenceFlag struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Default  string `json:"default"`
	Usage    string `json:"usage"`
	Required bool   `json:"required"`
}

func formatFlagDefault(def string) string {
	if def == "" || def == "[]" {
		return "-"
	}
	return def
}

func collectReferenceFlags(local, persistent *pflag.FlagSet, inheritPersistent bool) []referenceFlag {
	var flags []referenceFlag
	local.VisitAll(func(f *pflag.Flag) {
		if f.Hidden {
			return
		}
		flags = append(flags, referenceFlag{
			Name:     f.Name,
			Type:     f.Value.Type(),
			Default:  formatFlagDefault(f.DefValue),
			Usage:    f.Usage,
			Required: strings.Contains(strings.ToLower(f.Usage), "required"),
		})
	})
	if inheritPersistent {
		persistent.VisitAll(func(f *pflag.Flag) {
			if f.Hidden {
				return
			}
			if local.Lookup(f.Name) != nil {
				return
			}
			flags = append(flags, referenceFlag{
				Name:     f.Name,
				Type:     f.Value.Type(),
				Default:  formatFlagDefault(f.DefValue),
				Usage:    f.Usage,
				Required: strings.Contains(strings.ToLower(f.Usage), "required"),
			})
		})
	}
	return flags
}

type commandReference struct {
	Path             string             `json:"path"`
	Use              string             `json:"use"`
	Short            string             `json:"short,omitempty"`
	Type             string             `json:"type"`
	PermissionTier   string             `json:"permission_tier"`
	BlastRadius      string             `json:"blast_radius,omitempty"`
	Flags            []referenceFlag    `json:"flags,omitempty"`
	SupportedFormats []string           `json:"supported_formats"`
	Write            bool               `json:"write,omitempty"`
	OutputSchema     string             `json:"output_schema,omitempty"`
	Examples         []string           `json:"examples,omitempty"`
	Subcommands      []commandReference `json:"subcommands,omitempty"`
}

// referenceDataSchema is a machine-readable description of a command's data
// payload (the value inside the success envelope's data field, ignoring the
// envelope itself). shape is "object" or "array"; for arrays, fields describes
// one element. untrusted_fields lists keys carrying external, unsanitized data
// that an agent must treat as content, never as instructions.
type referenceDataSchema struct {
	Shape           string   `json:"shape"`
	Fields          []string `json:"fields"`
	UntrustedFields []string `json:"untrusted_fields,omitempty"`
}

type referenceDocument struct {
	Tool             string                         `json:"tool"`
	Version          string                         `json:"version"`
	SecurityTier     string                         `json:"security_tier"`
	ReleaseReadiness releaseReadiness               `json:"release_readiness"`
	Root             commandReference               `json:"root"`
	Commands         []commandReference             `json:"commands"`
	ExitCodes        map[string]string              `json:"exit_codes"`
	ErrorCodes       map[string]string              `json:"error_codes"`
	Schemas          map[string]referenceDataSchema `json:"schemas"`
}

var referenceCmd = &cobra.Command{
	Use:   "reference",
	Short: "Print all commands and flags in a structured format",
	Long:  "Outputs every command, subcommand, and flag in a machine-parseable format. Designed for AI Agents and script integration.",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if jsonMode {
			output.PrintJSON(buildReferenceDocument(rootCmd))
			return nil
		}
		printReference(cmd, rootCmd)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(referenceCmd)
}

func printReference(cmd *cobra.Command, root *cobra.Command) {
	var lines []string
	lines = append(lines, "# jira-cli Command Reference")
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("Version: %s", root.Version))
	lines = append(lines, "")

	walkCommands(root, &lines, "")

	for _, line := range lines {
		cmd.Println(line)
	}
}

func buildReferenceDocument(root *cobra.Command) referenceDocument {
	var commands []commandReference
	rootRef := buildCommandReference(root, "", &commands)
	return referenceDocument{
		Tool:             "jira-cli",
		Version:          root.Version,
		SecurityTier:     "T1",
		ReleaseReadiness: buildReleaseReadiness(),
		Root:             rootRef,
		Commands:         commands,
		ExitCodes:        referenceExitCodes(),
		ErrorCodes:       referenceErrorCodes(),
		Schemas:          referenceSchemas(),
	}
}

func buildCommandReference(cmd *cobra.Command, prefix string, commands *[]commandReference) commandReference {
	meta := commandMetaFor(cmd.CommandPath())
	ref := commandReference{
		Path:             strings.TrimSpace(prefix + cmd.Name()),
		Use:              cmd.Use,
		Short:            cmd.Short,
		Type:             commandType(cmd),
		PermissionTier:   permissionTier(cmd),
		BlastRadius:      blastRadius(cmd),
		Flags:            collectReferenceFlags(cmd.LocalFlags(), cmd.PersistentFlags(), cmd.Parent() != nil),
		SupportedFormats: supportedFormatsForCommand(cmd),
		Write:            isWriteCommand(cmd),
		OutputSchema:     meta.schema,
		Examples:         meta.examples,
	}
	children := cmd.Commands()
	sort.Slice(children, func(i, j int) bool {
		return children[i].Name() < children[j].Name()
	})
	for _, child := range children {
		if child.Hidden || !child.IsAvailableCommand() {
			continue
		}
		childRef := buildCommandReference(child, ref.Path+" ", commands)
		ref.Subcommands = append(ref.Subcommands, childRef)
	}
	if cmd.Runnable() {
		*commands = append(*commands, ref)
	}
	return ref
}

func supportedFormatsForCommand(cmd *cobra.Command) []string {
	formats := []string{outputFormatJSON, outputFormatText}
	if supportsRawFormat(cmd) {
		formats = append(formats, outputFormatRaw)
	}
	return formats
}

func commandType(cmd *cobra.Command) string {
	if isWriteCommand(cmd) {
		return "write"
	}
	return "read"
}

func permissionTier(cmd *cobra.Command) string {
	if !isWriteCommand(cmd) {
		return "read"
	}
	// Read from the same set the runtime gate enforces, so the advertised tier
	// can never drift from what --dangerous actually guards.
	if isDangerousCommand(cmd) {
		return "write-dangerous"
	}
	return "write"
}

func blastRadius(cmd *cobra.Command) string {
	if !isWriteCommand(cmd) {
		return "reads Jira data visible to the configured account"
	}
	switch cmd.CommandPath() {
	case "jira-cli issue delete":
		return "deletes one Jira issue visible and deletable by the configured account"
	case "jira-cli issue bulk-transition":
		return "transitions multiple Jira issues selected by explicit keys or JQL"
	case "jira-cli issue bulk-create":
		return "creates multiple Jira issues from a file via the native bulk endpoint"
	case "jira-cli issue backlog-move":
		return "moves multiple Jira issues out of their sprint back to the backlog"
	case "jira-cli issue bulk-assign":
		return "assigns or unassigns multiple Jira issues selected by explicit keys or JQL"
	case "jira-cli issue bulk-edit":
		return "applies the same field changes to multiple Jira issues selected by explicit keys or JQL"
	case "jira-cli issue rank":
		return "reorders multiple Jira issues relative to an anchor issue"
	case "jira-cli sprint close":
		return "closes one Jira sprint"
	case "jira-cli filter delete":
		return "deletes one saved Jira filter visible and deletable by the configured account"
	default:
		return "modifies Jira state within the configured account permissions"
	}
}

// commandMeta binds a command to its data-schema label (a key into
// referenceSchemas) and one runnable example per leaf. Keeping this beside the
// schema catalog makes `reference` a real, agent-usable source of truth instead
// of a generic stub.
type commandMeta struct {
	schema   string
	examples []string
}

// commandMetaFor returns the schema label and examples for a command path
// (e.g. "jira-cli issue get"). Non-leaf/grouping commands have no entry and
// return a zero value, which keeps output_schema and examples omitted.
func commandMetaFor(commandPath string) commandMeta {
	return commandMetaCatalog()[commandPath]
}

// commandMetaCatalog maps every leaf command path to its output schema label and
// a runnable example. Examples use --compact for single-line JSON. Write commands
// show the dry-run then confirm form; write-dangerous commands (issue delete,
// issue bulk-transition, sprint close, filter delete) include --dangerous in both
// steps, matching the runtime second gate.
func commandMetaCatalog() map[string]commandMeta {
	return map[string]commandMeta{
		// Boards.
		"jira-cli board list":    {schema: "board[]", examples: []string{"jira-cli board list --compact"}},
		"jira-cli board get":     {schema: "board", examples: []string{"jira-cli board get 1 --compact"}},
		"jira-cli board sprints": {schema: "sprint[]", examples: []string{"jira-cli board sprints 1 --compact"}},
		"jira-cli board backlog": {schema: "issue[]", examples: []string{"jira-cli board backlog 1 --compact"}},
		"jira-cli board epics":   {schema: "issue[]", examples: []string{"jira-cli board epics 1 --compact"}},

		// Epics.
		"jira-cli epic list":   {schema: "issue[]", examples: []string{"jira-cli epic list --project PROJ --compact"}},
		"jira-cli epic issues": {schema: "issue[]", examples: []string{"jira-cli epic issues PROJ-1 --compact"}},

		// Issues (reads).
		"jira-cli issue get":          {schema: "issue", examples: []string{"jira-cli issue get PROJ-1 --compact"}},
		"jira-cli issue list":         {schema: "issue[]", examples: []string{"jira-cli issue list --project PROJ --compact"}},
		"jira-cli issue transitions":  {schema: "transition[]", examples: []string{"jira-cli issue transitions PROJ-1 --compact"}},
		"jira-cli issue link-types":   {schema: "link_type[]", examples: []string{"jira-cli issue link-types --compact"}},
		"jira-cli issue watchers":     {schema: "user[]", examples: []string{"jira-cli issue watchers PROJ-1 --compact"}},
		"jira-cli issue attachments":  {schema: "attachment[]", examples: []string{"jira-cli issue attachments PROJ-1 --compact"}},
		"jira-cli issue remote-links": {schema: "remote_link[]", examples: []string{"jira-cli issue remote-links PROJ-1 --compact"}},
		"jira-cli issue link list":    {schema: "issue_link_row[]", examples: []string{"jira-cli issue link list PROJ-1 --compact"}},
		"jira-cli issue comment list": {schema: "comment[]", examples: []string{"jira-cli issue comment list PROJ-1 --compact"}},
		"jira-cli issue worklog list": {schema: "worklog[]", examples: []string{"jira-cli issue worklog list PROJ-1 --compact"}},

		// Issues (writes, non-dangerous): dry-run then confirm.
		"jira-cli issue create":         {schema: "create_result", examples: []string{"jira-cli issue create --project PROJ --type Task --summary 'Fix login' --dry-run --compact", "jira-cli issue create --project PROJ --type Task --summary 'Fix login' --confirm <token> --compact"}},
		"jira-cli issue clone":          {schema: "clone_result", examples: []string{"jira-cli issue clone PROJ-1 --dry-run --compact", "jira-cli issue clone PROJ-1 --confirm <token> --compact"}},
		"jira-cli issue edit":           {schema: "edit_result", examples: []string{"jira-cli issue edit PROJ-1 --summary 'Updated' --dry-run --compact", "jira-cli issue edit PROJ-1 --summary 'Updated' --confirm <token> --compact"}},
		"jira-cli issue transition":     {schema: "transition_result", examples: []string{"jira-cli issue transition PROJ-1 --to Done --dry-run --compact", "jira-cli issue transition PROJ-1 --to Done --confirm <token> --compact"}},
		"jira-cli issue assign":         {schema: "assign_result", examples: []string{"jira-cli issue assign PROJ-1 --user jdoe --dry-run --compact", "jira-cli issue assign PROJ-1 --user jdoe --confirm <token> --compact"}},
		"jira-cli issue unassign":       {schema: "assign_result", examples: []string{"jira-cli issue unassign PROJ-1 --dry-run --compact", "jira-cli issue unassign PROJ-1 --confirm <token> --compact"}},
		"jira-cli issue watch":          {schema: "issue_action_result", examples: []string{"jira-cli issue watch PROJ-1 --dry-run --compact", "jira-cli issue watch PROJ-1 --confirm <token> --compact"}},
		"jira-cli issue unwatch":        {schema: "issue_action_result", examples: []string{"jira-cli issue unwatch PROJ-1 --dry-run --compact", "jira-cli issue unwatch PROJ-1 --confirm <token> --compact"}},
		"jira-cli issue vote":           {schema: "issue_action_result", examples: []string{"jira-cli issue vote PROJ-1 --dry-run --compact", "jira-cli issue vote PROJ-1 --confirm <token> --compact"}},
		"jira-cli issue unvote":         {schema: "issue_action_result", examples: []string{"jira-cli issue unvote PROJ-1 --dry-run --compact", "jira-cli issue unvote PROJ-1 --confirm <token> --compact"}},
		"jira-cli issue link":           {schema: "link_result", examples: []string{"jira-cli issue link PROJ-1 --to PROJ-2 --type Blocks --dry-run --compact", "jira-cli issue link PROJ-1 --to PROJ-2 --type Blocks --confirm <token> --compact"}},
		"jira-cli issue unlink":         {schema: "unlink_result", examples: []string{"jira-cli issue unlink 10001 --dry-run --compact", "jira-cli issue unlink 10001 --confirm <token> --compact"}},
		"jira-cli issue attach":         {schema: "attachment[]", examples: []string{"jira-cli issue attach PROJ-1 --file ./log.txt --dry-run --compact", "jira-cli issue attach PROJ-1 --file ./log.txt --confirm <token> --compact"}},
		"jira-cli issue remote-link":    {schema: "remote_link", examples: []string{"jira-cli issue remote-link PROJ-1 --url https://example.com --title Ref --dry-run --compact", "jira-cli issue remote-link PROJ-1 --url https://example.com --title Ref --confirm <token> --compact"}},
		"jira-cli issue comment add":    {schema: "comment", examples: []string{"jira-cli issue comment add PROJ-1 --body 'Looking into it' --dry-run --compact", "jira-cli issue comment add PROJ-1 --body 'Looking into it' --confirm <token> --compact"}},
		"jira-cli issue comment delete": {schema: "comment_delete_result", examples: []string{"jira-cli issue comment delete PROJ-1 --id 10000 --dry-run --compact", "jira-cli issue comment delete PROJ-1 --id 10000 --confirm <token> --compact"}},
		"jira-cli issue worklog add":    {schema: "worklog", examples: []string{"jira-cli issue worklog add PROJ-1 --time 1h --dry-run --compact", "jira-cli issue worklog add PROJ-1 --time 1h --confirm <token> --compact"}},

		// Issues (batch writes): plural inputs, dry-run then confirm, aggregated items[]/summary.
		"jira-cli issue bulk-create":  {schema: "batch_create_result", examples: []string{"jira-cli issue bulk-create --file issues.json --dry-run --compact", "jira-cli issue bulk-create --file issues.json --confirm <token> --compact"}},
		"jira-cli issue backlog-move": {schema: "batch_result", examples: []string{"jira-cli issue backlog-move --issues PROJ-1,PROJ-2 --dry-run --compact", "jira-cli issue backlog-move --issues PROJ-1,PROJ-2 --confirm <token> --compact"}},
		"jira-cli issue bulk-assign":  {schema: "batch_result", examples: []string{"jira-cli issue bulk-assign --issues PROJ-1,PROJ-2 --assignee jdoe --dry-run --compact", "jira-cli issue bulk-assign --issues PROJ-1,PROJ-2 --assignee jdoe --confirm <token> --compact"}},
		"jira-cli issue bulk-edit":    {schema: "batch_result", examples: []string{"jira-cli issue bulk-edit --issues PROJ-1,PROJ-2 --priority High --dry-run --compact", "jira-cli issue bulk-edit --issues PROJ-1,PROJ-2 --priority High --confirm <token> --compact"}},
		"jira-cli issue rank":         {schema: "batch_result", examples: []string{"jira-cli issue rank --issues PROJ-1,PROJ-2 --after PROJ-9 --dry-run --compact", "jira-cli issue rank --issues PROJ-1,PROJ-2 --after PROJ-9 --confirm <token> --compact"}},

		// Issues (write-dangerous): --dangerous in both steps.
		"jira-cli issue delete":          {schema: "delete_result", examples: []string{"jira-cli issue delete PROJ-1 --dangerous --dry-run --compact", "jira-cli issue delete PROJ-1 --dangerous --confirm <token> --compact"}},
		"jira-cli issue bulk-transition": {schema: "bulk_transition_result", examples: []string{"jira-cli issue bulk-transition Done --issues PROJ-1,PROJ-2 --dangerous --dry-run --compact", "jira-cli issue bulk-transition Done --issues PROJ-1,PROJ-2 --dangerous --confirm <token> --compact"}},

		// Filters.
		"jira-cli filter list":   {schema: "filter[]", examples: []string{"jira-cli filter list --compact"}},
		"jira-cli filter get":    {schema: "filter", examples: []string{"jira-cli filter get 10000 --compact"}},
		"jira-cli filter run":    {schema: "search_page", examples: []string{"jira-cli filter run 10000 --compact"}},
		"jira-cli filter create": {schema: "filter", examples: []string{"jira-cli filter create --name MyFilter --jql 'project = PROJ' --dry-run --compact", "jira-cli filter create --name MyFilter --jql 'project = PROJ' --confirm <token> --compact"}},
		"jira-cli filter delete": {schema: "filter_delete_result", examples: []string{"jira-cli filter delete 10000 --dangerous --dry-run --compact", "jira-cli filter delete 10000 --dangerous --confirm <token> --compact"}},

		// Projects.
		"jira-cli project list":        {schema: "project[]", examples: []string{"jira-cli project list --compact"}},
		"jira-cli project get":         {schema: "project", examples: []string{"jira-cli project get PROJ --compact"}},
		"jira-cli project components":  {schema: "component[]", examples: []string{"jira-cli project components PROJ --compact"}},
		"jira-cli project versions":    {schema: "version[]", examples: []string{"jira-cli project versions PROJ --compact"}},
		"jira-cli project issue-types": {schema: "issue_type[]", examples: []string{"jira-cli project issue-types PROJ --compact"}},
		"jira-cli project fields":      {schema: "field[]", examples: []string{"jira-cli project fields PROJ --compact"}},

		// Search.
		"jira-cli search": {schema: "search_page", examples: []string{"jira-cli search 'project = PROJ AND status = Open' --compact"}},

		// Sprints.
		"jira-cli sprint list":   {schema: "sprint[]", examples: []string{"jira-cli sprint list --board 1 --compact"}},
		"jira-cli sprint active": {schema: "sprint_with_issues[]", examples: []string{"jira-cli sprint active --board 1 --compact"}},
		"jira-cli sprint issues": {schema: "issue[]", examples: []string{"jira-cli sprint issues 10 --compact"}},
		"jira-cli sprint create": {schema: "sprint", examples: []string{"jira-cli sprint create --board 1 --name 'Sprint 5' --dry-run --compact", "jira-cli sprint create --board 1 --name 'Sprint 5' --confirm <token> --compact"}},
		"jira-cli sprint update": {schema: "sprint", examples: []string{"jira-cli sprint update --sprint 10 --name 'Renamed' --dry-run --compact", "jira-cli sprint update --sprint 10 --name 'Renamed' --confirm <token> --compact"}},
		"jira-cli sprint move":   {schema: "sprint_move_result", examples: []string{"jira-cli sprint move --sprint 10 --issues PROJ-1,PROJ-2 --dry-run --compact", "jira-cli sprint move --sprint 10 --issues PROJ-1,PROJ-2 --confirm <token> --compact"}},
		"jira-cli sprint report": {schema: "sprint_report", examples: []string{"jira-cli sprint report 10 --board 1 --compact"}},
		"jira-cli sprint close":  {schema: "sprint_close_result", examples: []string{"jira-cli sprint close --sprint 10 --dangerous --dry-run --compact", "jira-cli sprint close --sprint 10 --dangerous --confirm <token> --compact"}},

		// Users.
		"jira-cli user me":     {schema: "user", examples: []string{"jira-cli user me --compact"}},
		"jira-cli user search": {schema: "user[]", examples: []string{"jira-cli user search jdoe --compact"}},

		// Auth.
		"jira-cli login":  {schema: "auth_result", examples: []string{"jira-cli login --host https://jira.example.com --token <pat> --compact"}},
		"jira-cli logout": {schema: "logout_result", examples: []string{"jira-cli logout --compact"}},

		// Self-description.
		"jira-cli reference": {schema: "reference", examples: []string{"jira-cli reference --compact"}},
		"jira-cli context":   {schema: "context", examples: []string{"jira-cli context --compact"}},
		"jira-cli doctor":    {schema: "doctor", examples: []string{"jira-cli doctor --compact"}},
		"jira-cli changelog": {schema: "changelog", examples: []string{"jira-cli changelog --since 1.0.0 --compact"}},

		// Lifecycle (self-update): single command, no confirm token. --check and
		// --dry-run are optional read-only flags.
		"jira-cli update": {schema: "update_report", examples: []string{"jira-cli update --compact", "jira-cli update --check --compact", "jira-cli update --dry-run --compact"}},
	}
}

// referenceSchemas is the catalog of data-payload shapes that command
// output_schema labels point into. Field lists mirror the actual JSON emitted by
// each command (FlatIssue/FlatSprint/FlatBoard and the flat* structs in cmd, the
// raw api.* structs for passthrough commands, and the inline write-result maps).
// _untrusted entries match the runtime tagging an agent must honor.
func referenceSchemas() map[string]referenceDataSchema {
	return map[string]referenceDataSchema{
		// Flattened core resources (cmd/flatten.go, internal/output/flatten.go).
		"issue":    {Shape: "object", Fields: []string{"key", "summary", "description", "status", "type", "assignee", "reporter", "priority", "created", "updated", "labels", "component", "parent", "_untrusted"}, UntrustedFields: []string{"summary", "description"}},
		"issue[]":  {Shape: "array", Fields: []string{"key", "summary", "description", "status", "type", "assignee", "reporter", "priority", "created", "updated", "labels", "component", "parent", "_untrusted"}, UntrustedFields: []string{"summary", "description"}},
		"sprint":   {Shape: "object", Fields: []string{"id", "name", "state", "startDate", "endDate", "goal"}},
		"sprint[]": {Shape: "array", Fields: []string{"id", "name", "state", "startDate", "endDate", "goal"}},
		"board":    {Shape: "object", Fields: []string{"id", "name", "type", "projectKey", "displayName"}},
		"board[]":  {Shape: "array", Fields: []string{"id", "name", "type", "projectKey", "displayName"}},

		// Comments, worklogs, attachments (cmd/issue_comment.go, issue_worklog.go, issue_attach.go).
		"comment":      {Shape: "object", Fields: []string{"id", "author", "created", "updated", "body", "_untrusted"}, UntrustedFields: []string{"body"}},
		"comment[]":    {Shape: "array", Fields: []string{"id", "author", "created", "updated", "body", "_untrusted"}, UntrustedFields: []string{"body"}},
		"worklog":      {Shape: "object", Fields: []string{"id", "author", "started", "timeSpent", "timeSpentSeconds", "comment", "_untrusted"}, UntrustedFields: []string{"comment"}},
		"worklog[]":    {Shape: "array", Fields: []string{"id", "author", "started", "timeSpent", "timeSpentSeconds", "comment", "_untrusted"}, UntrustedFields: []string{"comment"}},
		"attachment[]": {Shape: "array", Fields: []string{"id", "filename", "size", "mimeType", "author", "created", "content", "_untrusted"}, UntrustedFields: []string{"filename"}},

		// Sprint active groups each sprint with its issues.
		"sprint_with_issues[]": {Shape: "array", Fields: []string{"sprint", "issues"}},

		// Issue link rows (cmd/flatten.go FlatIssueLink): one row per link.
		"issue_link_row[]": {Shape: "array", Fields: []string{"id", "direction", "type", "linkedKey", "linkedSummary", "linkedStatus", "_untrusted"}, UntrustedFields: []string{"linkedSummary"}},

		// Sprint velocity/completion report (api.SprintReport). source records
		// whether figures came from the Greenhopper chart or were computed.
		"sprint_report": {Shape: "object", Fields: []string{"sprintId", "sprintName", "state", "committedIssues", "completedIssues", "notCompletedIssues", "removedIssues", "committedPoints", "completedPoints", "notCompletedPoints", "source"}},

		// Search/filter run pagination wrapper (cmd/search.go, filter.go).
		"search_page": {Shape: "object", Fields: []string{"total", "startAt", "maxResults", "isLast", "nextStartAt", "issues"}},

		// Raw Jira API passthrough structs (internal/api/types.go).
		"filter":        {Shape: "object", Fields: []string{"id", "name", "description", "jql", "favourite", "sharePermissions"}},
		"filter[]":      {Shape: "array", Fields: []string{"id", "name", "description", "jql", "favourite", "sharePermissions"}},
		"project":       {Shape: "object", Fields: []string{"id", "key", "name", "projectTypeKey", "lead", "description", "url", "issueTypes", "style"}},
		"project[]":     {Shape: "array", Fields: []string{"id", "key", "name", "projectTypeKey", "lead", "description", "url", "issueTypes", "style"}},
		"component[]":   {Shape: "array", Fields: []string{"id", "name"}},
		"version[]":     {Shape: "array", Fields: []string{"id", "name", "released", "releaseDate", "description"}},
		"issue_type[]":  {Shape: "array", Fields: []string{"id", "name", "subtask"}},
		"field[]":       {Shape: "array", Fields: []string{"id", "key", "name", "custom", "navigable", "searchable", "clauseNames", "schema"}},
		"user":          {Shape: "object", Fields: []string{"name", "displayName", "emailAddress", "active"}},
		"user[]":        {Shape: "array", Fields: []string{"name", "displayName", "emailAddress", "active"}},
		"transition[]":  {Shape: "array", Fields: []string{"id", "name", "to"}},
		"link_type[]":   {Shape: "array", Fields: []string{"id", "name", "inward", "outward"}},
		"remote_link":   {Shape: "object", Fields: []string{"id", "object"}},
		"remote_link[]": {Shape: "array", Fields: []string{"id", "object"}},

		// Inline write-result maps (cmd/issue*.go, filter.go, sprint.go, login.go).
		"create_result":          {Shape: "object", Fields: []string{"key", "id", "url"}},
		"clone_result":           {Shape: "object", Fields: []string{"key", "id", "url", "clonedFrom", "linkErrors"}},
		"edit_result":            {Shape: "object", Fields: []string{"key", "status", "warning"}},
		"transition_result":      {Shape: "object", Fields: []string{"issueKey", "fromStatus", "toStatus"}},
		"assign_result":          {Shape: "object", Fields: []string{"issueKey", "assignee"}},
		"issue_action_result":    {Shape: "object", Fields: []string{"issueKey", "action"}},
		"link_result":            {Shape: "object", Fields: []string{"from", "to", "type", "status"}},
		"unlink_result":          {Shape: "object", Fields: []string{"linkId", "status"}},
		"delete_result":          {Shape: "object", Fields: []string{"key", "status"}},
		"comment_delete_result":  {Shape: "object", Fields: []string{"issueKey", "commentId", "status"}},
		"filter_delete_result":   {Shape: "object", Fields: []string{"filterId", "status"}},
		"bulk_transition_result": {Shape: "object", Fields: []string{"targetStatus", "total", "success", "skipped", "failed", "results"}},

		// Batch results (cmd/bulk_common.go): one aggregated items[]/summary for
		// every batch command, class A or B. items[].target is the natural input
		// key (issue key, or 0-based file index for bulk-create); items[].error
		// carries the same {code,retryable} taxonomy as the top-level envelope.
		// error.message relays upstream Jira text, so it is _untrusted content.
		"batch_result":        {Shape: "object", Fields: []string{"action", "items", "summary", "_untrusted"}, UntrustedFields: []string{"items[].error.message"}},
		"batch_create_result": {Shape: "object", Fields: []string{"action", "items", "summary", "_untrusted"}, UntrustedFields: []string{"items[].error.message"}},
		"sprint_move_result":  {Shape: "object", Fields: []string{"sprintId", "issues"}},
		"sprint_close_result": {Shape: "object", Fields: []string{"sprintId", "state", "changed"}},
		"auth_result":         {Shape: "object", Fields: []string{"status", "display_name", "username"}},
		"logout_result":       {Shape: "object", Fields: []string{"status"}},

		// Self-description documents (cmd/reference.go, context.go, doctor.go, changelog.go, update.go).
		"reference":     {Shape: "object", Fields: []string{"tool", "version", "security_tier", "release_readiness", "root", "commands", "exit_codes", "error_codes", "schemas"}},
		"context":       {Shape: "object", Fields: []string{"tool", "version", "runtime", "config", "credentials", "account", "errors", "env", "notices"}},
		"doctor":        {Shape: "object", Fields: []string{"checks", "notices", "host", "username", "display_name", "latency_ms"}},
		"changelog":     {Shape: "object", Fields: []string{"current_version", "since", "entries"}},
		"update_report": {Shape: "object", Fields: []string{"current_version", "latest_version", "requested_version", "previous_version", "knowledge_refresh", "update_available", "installed", "check_only", "dry_run", "install_method", "manager_command", "asset", "path", "checksum_verified", "signature_status", "signature_verified", "skill_sync_command", "skill_sync_status", "notices"}},
	}
}

func referenceExitCodes() map[string]string {
	return map[string]string{
		"0": "success",
		"1": "generic error",
		"2": "argument or validation error",
		"3": "resource not found",
		"4": "auth, permission, or config failure",
		"5": "confirmation token required",
		"6": "conflict or invalid confirmation token",
		"7": "retryable transient error",
		"8": "timeout",
	}
}

func referenceErrorCodes() map[string]string {
	return map[string]string{
		string(output.ErrConfig):          "configuration is missing or invalid",
		string(output.ErrAuth):            "authentication failed",
		string(output.ErrForbidden):       "permission denied",
		string(output.ErrNotFound):        "resource not found",
		string(output.ErrRateLimit):       "rate limited",
		string(output.ErrServer):          "Jira server error",
		string(output.ErrValidation):      "arguments or request are invalid",
		string(output.ErrNetwork):         "network failure",
		string(output.ErrConfirmRequired): "write requires a dry-run confirmation token",
		string(output.ErrConflict):        "confirmation token expired or no longer matches",
		string(output.ErrTimeout):         "request timed out",
		string(output.ErrUnknown):         "unclassified error",
	}
}

func walkCommands(cmd *cobra.Command, lines *[]string, prefix string) {
	if cmd.Hidden {
		return
	}

	name := prefix + cmd.Use
	*lines = append(*lines, "## "+name)
	*lines = append(*lines, "")
	if cmd.Short != "" {
		*lines = append(*lines, cmd.Short)
		*lines = append(*lines, "")
	}

	// Surface the leaf command's data schema and runnable example so the text
	// reference stays consistent with the JSON reference's source of truth.
	if meta := commandMetaFor(cmd.CommandPath()); meta.schema != "" {
		*lines = append(*lines, fmt.Sprintf("Data schema: `%s`", meta.schema))
		*lines = append(*lines, "")
		if len(meta.examples) > 0 {
			*lines = append(*lines, "Examples:")
			*lines = append(*lines, "")
			for _, ex := range meta.examples {
				*lines = append(*lines, "    "+ex)
			}
			*lines = append(*lines, "")
		}
	}

	flags := collectReferenceFlags(cmd.LocalFlags(), cmd.PersistentFlags(), cmd.Parent() != nil)

	if len(flags) > 0 {
		*lines = append(*lines, "### Flags")
		*lines = append(*lines, "")
		*lines = append(*lines, "| Flag | Type | Default | Description |")
		*lines = append(*lines, "|------|------|---------|-------------|")
		for _, f := range flags {
			*lines = append(*lines, fmt.Sprintf("| `--%s` | %s | %s | %s |", f.Name, f.Type, f.Default, f.Usage))
		}
		*lines = append(*lines, "")
	}

	children := cmd.Commands()
	if len(children) > 0 {
		sort.Slice(children, func(i, j int) bool {
			return children[i].Name() < children[j].Name()
		})
		for _, child := range children {
			if !child.Hidden && child.IsAvailableCommand() {
				walkCommands(child, lines, prefix+cmd.Name()+" ")
			}
		}
	}
}
