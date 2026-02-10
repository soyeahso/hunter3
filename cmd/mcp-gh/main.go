package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// JSON-RPC types

type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type JSONRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
}

type RPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema InputSchema `json:"inputSchema"`
}

type InputSchema struct {
	Type       string              `json:"type"`
	Properties map[string]Property `json:"properties"`
	Required   []string            `json:"required,omitempty"`
}

type Property struct {
	Type        string    `json:"type"`
	Description string    `json:"description"`
	Items       *ItemType `json:"items,omitempty"`
	Enum        []string  `json:"enum,omitempty"`
	Default     string    `json:"default,omitempty"`
	Minimum     *int      `json:"minimum,omitempty"`
	Maximum     *int      `json:"maximum,omitempty"`
}

type ItemType struct {
	Type string `json:"type"`
}

type CallToolParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

type ToolResult struct {
	Content []ContentItem `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

type ContentItem struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type InitializeResult struct {
	ProtocolVersion string       `json:"protocolVersion"`
	Capabilities    Capabilities `json:"capabilities"`
	ServerInfo      ServerInfo   `json:"serverInfo"`
}

type Capabilities struct {
	Tools map[string]interface{} `json:"tools"`
}

type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type ListToolsResult struct {
	Tools []Tool `json:"tools"`
}

// GhResult is returned from executeGhCommand as JSON.
type GhResult struct {
	Command string `json:"command"`
	Success bool   `json:"success"`
	Stdout  string `json:"stdout,omitempty"`
	Stderr  string `json:"stderr,omitempty"`
	Error   string `json:"error,omitempty"`
}

// Helper constructors for schema properties

func stringProp(desc string) Property {
	return Property{Type: "string", Description: desc}
}

func stringPropDefault(desc, def string) Property {
	return Property{Type: "string", Description: desc, Default: def}
}

func stringArrayProp(desc string) Property {
	return Property{Type: "array", Description: desc, Items: &ItemType{Type: "string"}}
}

func intProp(desc string, min, max int) Property {
	return Property{Type: "number", Description: desc, Minimum: &min, Maximum: &max}
}

// MCPServer handles the JSON-RPC stdin/stdout protocol.
type MCPServer struct{}

var logger *log.Logger

func initLogger() {
	// Create logs directory if it doesn't exist
	logsDir := filepath.Join(os.Getenv("HOME"), ".hunter3", "logs")
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create logs directory: %v\n", err)
		return
	}

	// Open log file
	logFile := filepath.Join(logsDir, "mcp-gh.log")
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open log file: %v\n", err)
		return
	}

	// Create logger that writes to both file and stderr
	logger = log.New(io.MultiWriter(f, os.Stderr), "[mcp-gh] ", log.LstdFlags)
	logger.Println("MCP GitHub CLI server starting...")
}

func main() {
	initLogger()
	initAllowedPaths()
	s := &MCPServer{}
	logger.Println("Server initialized")
	s.Run()
}

func (s *MCPServer) Run() {
	scanner := bufio.NewScanner(os.Stdin)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	logger.Println("Listening for requests on stdin...")

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		logger.Printf("Received request: %s\n", line)
		s.handleRequest(line)
	}
	if err := scanner.Err(); err != nil && err != io.EOF {
		logger.Printf("Error reading stdin: %v\n", err)
		fmt.Fprintf(os.Stderr, "Error reading stdin: %v\n", err)
	}
	logger.Println("Server shutting down")
}

func (s *MCPServer) handleRequest(line string) {
	var req JSONRPCRequest
	if err := json.Unmarshal([]byte(line), &req); err != nil {
		logger.Printf("Parse error: %v\n", err)
		s.sendError(nil, -32700, "Parse error", err.Error())
		return
	}

	logger.Printf("Handling method: %s\n", req.Method)

	switch req.Method {
	case "initialize":
		s.handleInitialize(req)
	case "tools/list":
		s.handleListTools(req)
	case "tools/call":
		s.handleCallTool(req)
	case "notifications/initialized":
		// no-op
		logger.Println("Received initialized notification")
	default:
		logger.Printf("Unknown method: %s\n", req.Method)
		s.sendError(req.ID, -32601, "Method not found", fmt.Sprintf("Unknown method: %s", req.Method))
	}
}

func (s *MCPServer) handleInitialize(req JSONRPCRequest) {
	logger.Println("Handling initialize request")
	s.sendResponse(req.ID, InitializeResult{
		ProtocolVersion: "2024-11-05",
		Capabilities:    Capabilities{Tools: map[string]interface{}{}},
		ServerInfo:      ServerInfo{Name: "mcp-gh", Version: "1.0.0"},
	})
}

// ---------- Tool definitions ----------

func (s *MCPServer) handleListTools(req JSONRPCRequest) {
	logger.Println("Handling list tools request")
	flagsProp := stringArrayProp("Additional flags passed directly to the gh command")
	repoProp := stringProp("Repository path (working directory for the command)")

	tools := []Tool{
		// --- Repository operations ---
		{
			Name:        "gh_repo_view",
			Description: "View repository information. Can view current repo or specify owner/repo.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"repository_path": repoProp,
					"repo":            stringProp("Repository in OWNER/REPO format (optional, uses current repo if not specified)"),
					"web":             stringProp("Open repository in browser (true/false)"),
					"flags":           flagsProp,
				},
			},
		},
		{
			Name:        "gh_repo_clone",
			Description: "Clone a repository locally.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"repo":  stringProp("Repository to clone (OWNER/REPO or URL)"),
					"path":  stringProp("Local path to clone into (optional)"),
					"flags": flagsProp,
				},
				Required: []string{"repo"},
			},
		},
		{
			Name:        "gh_repo_create",
			Description: "Create a new repository.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"name":        stringProp("Repository name"),
					"description": stringProp("Repository description"),
					"public":      stringProp("Make repository public (true/false)"),
					"flags":       flagsProp,
				},
				Required: []string{"name"},
			},
		},
		{
			Name:        "gh_repo_fork",
			Description: "Fork a repository.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"repo":  stringProp("Repository to fork (OWNER/REPO)"),
					"clone": stringProp("Clone the fork locally (true/false)"),
					"flags": flagsProp,
				},
				Required: []string{"repo"},
			},
		},
		{
			Name:        "gh_repo_list",
			Description: "List repositories for a user or organization.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"owner": stringProp("Owner (username or organization)"),
					"limit": intProp("Maximum number of repositories to list", 1, 1000),
					"flags": flagsProp,
				},
			},
		},

		// --- Issue operations ---
		{
			Name:        "gh_issue_list",
			Description: "List issues in a repository.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"repository_path": repoProp,
					"repo":            stringProp("Repository in OWNER/REPO format (optional)"),
					"state":           stringProp("Issue state: open, closed, or all"),
					"assignee":        stringProp("Filter by assignee"),
					"label":           stringProp("Filter by label"),
					"limit":           intProp("Maximum number of issues to list", 1, 1000),
					"flags":           flagsProp,
				},
			},
		},
		{
			Name:        "gh_issue_view",
			Description: "View an issue.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"repository_path": repoProp,
					"number":          stringProp("Issue number"),
					"repo":            stringProp("Repository in OWNER/REPO format (optional)"),
					"web":             stringProp("Open issue in browser (true/false)"),
					"flags":           flagsProp,
				},
				Required: []string{"number"},
			},
		},
		{
			Name:        "gh_issue_create",
			Description: "Create a new issue.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"repository_path": repoProp,
					"title":           stringProp("Issue title"),
					"body":            stringProp("Issue body"),
					"assignee":        stringProp("Assignee username"),
					"label":           stringArrayProp("Labels to add"),
					"repo":            stringProp("Repository in OWNER/REPO format (optional)"),
					"flags":           flagsProp,
				},
				Required: []string{"title"},
			},
		},
		{
			Name:        "gh_issue_close",
			Description: "Close an issue.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"repository_path": repoProp,
					"number":          stringProp("Issue number"),
					"repo":            stringProp("Repository in OWNER/REPO format (optional)"),
					"flags":           flagsProp,
				},
				Required: []string{"number"},
			},
		},
		{
			Name:        "gh_issue_reopen",
			Description: "Reopen an issue.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"repository_path": repoProp,
					"number":          stringProp("Issue number"),
					"repo":            stringProp("Repository in OWNER/REPO format (optional)"),
					"flags":           flagsProp,
				},
				Required: []string{"number"},
			},
		},

		// --- Pull Request operations ---
		{
			Name:        "gh_pr_list",
			Description: "List pull requests in a repository.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"repository_path": repoProp,
					"repo":            stringProp("Repository in OWNER/REPO format (optional)"),
					"state":           stringProp("PR state: open, closed, merged, or all"),
					"author":          stringProp("Filter by author"),
					"assignee":        stringProp("Filter by assignee"),
					"label":           stringProp("Filter by label"),
					"limit":           intProp("Maximum number of PRs to list", 1, 1000),
					"flags":           flagsProp,
				},
			},
		},
		{
			Name:        "gh_pr_view",
			Description: "View a pull request.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"repository_path": repoProp,
					"number":          stringProp("PR number"),
					"repo":            stringProp("Repository in OWNER/REPO format (optional)"),
					"web":             stringProp("Open PR in browser (true/false)"),
					"flags":           flagsProp,
				},
				Required: []string{"number"},
			},
		},
		{
			Name:        "gh_pr_create",
			Description: "Create a pull request.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"repository_path": repoProp,
					"title":           stringProp("PR title"),
					"body":            stringProp("PR body"),
					"base":            stringProp("Base branch"),
					"head":            stringProp("Head branch"),
					"draft":           stringProp("Create as draft (true/false)"),
					"assignee":        stringProp("Assignee username"),
					"label":           stringArrayProp("Labels to add"),
					"repo":            stringProp("Repository in OWNER/REPO format (optional)"),
					"flags":           flagsProp,
				},
				Required: []string{"title"},
			},
		},
		{
			Name:        "gh_pr_checkout",
			Description: "Check out a pull request locally.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"repository_path": repoProp,
					"number":          stringProp("PR number"),
					"repo":            stringProp("Repository in OWNER/REPO format (optional)"),
					"flags":           flagsProp,
				},
				Required: []string{"number"},
			},
		},
		{
			Name:        "gh_pr_merge",
			Description: "Merge a pull request.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"repository_path": repoProp,
					"number":          stringProp("PR number"),
					"merge_method":    stringProp("Merge method: merge, squash, or rebase"),
					"delete_branch":   stringProp("Delete branch after merge (true/false)"),
					"repo":            stringProp("Repository in OWNER/REPO format (optional)"),
					"flags":           flagsProp,
				},
				Required: []string{"number"},
			},
		},
		{
			Name:        "gh_pr_close",
			Description: "Close a pull request.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"repository_path": repoProp,
					"number":          stringProp("PR number"),
					"delete_branch":   stringProp("Delete branch after closing (true/false)"),
					"repo":            stringProp("Repository in OWNER/REPO format (optional)"),
					"flags":           flagsProp,
				},
				Required: []string{"number"},
			},
		},
		{
			Name:        "gh_pr_review",
			Description: "Add a review to a pull request.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"repository_path": repoProp,
					"number":          stringProp("PR number"),
					"approve":         stringProp("Approve the PR (true/false)"),
					"request_changes": stringProp("Request changes (true/false)"),
					"comment":         stringProp("Review comment"),
					"body":            stringProp("Review body"),
					"repo":            stringProp("Repository in OWNER/REPO format (optional)"),
					"flags":           flagsProp,
				},
				Required: []string{"number"},
			},
		},
		{
			Name:        "gh_pr_diff",
			Description: "View changes in a pull request.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"repository_path": repoProp,
					"number":          stringProp("PR number"),
					"repo":            stringProp("Repository in OWNER/REPO format (optional)"),
					"flags":           flagsProp,
				},
				Required: []string{"number"},
			},
		},

		// --- Workflow/Actions operations ---
		{
			Name:        "gh_run_list",
			Description: "List workflow runs.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"repository_path": repoProp,
					"repo":            stringProp("Repository in OWNER/REPO format (optional)"),
					"workflow":        stringProp("Filter by workflow name or ID"),
					"limit":           intProp("Maximum number of runs to list", 1, 1000),
					"flags":           flagsProp,
				},
			},
		},
		{
			Name:        "gh_run_view",
			Description: "View a workflow run.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"repository_path": repoProp,
					"run_id":          stringProp("Workflow run ID"),
					"repo":            stringProp("Repository in OWNER/REPO format (optional)"),
					"log":             stringProp("View full log (true/false)"),
					"flags":           flagsProp,
				},
				Required: []string{"run_id"},
			},
		},
		{
			Name:        "gh_run_rerun",
			Description: "Rerun a workflow run.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"repository_path": repoProp,
					"run_id":          stringProp("Workflow run ID"),
					"repo":            stringProp("Repository in OWNER/REPO format (optional)"),
					"flags":           flagsProp,
				},
				Required: []string{"run_id"},
			},
		},
		{
			Name:        "gh_workflow_list",
			Description: "List workflows in a repository.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"repository_path": repoProp,
					"repo":            stringProp("Repository in OWNER/REPO format (optional)"),
					"flags":           flagsProp,
				},
			},
		},
		{
			Name:        "gh_workflow_run",
			Description: "Trigger a workflow run.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"repository_path": repoProp,
					"workflow":        stringProp("Workflow name or ID"),
					"ref":             stringProp("Branch or tag to run workflow on"),
					"repo":            stringProp("Repository in OWNER/REPO format (optional)"),
					"flags":           flagsProp,
				},
				Required: []string{"workflow"},
			},
		},

		// --- Release operations ---
		{
			Name:        "gh_release_list",
			Description: "List releases in a repository.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"repository_path": repoProp,
					"repo":            stringProp("Repository in OWNER/REPO format (optional)"),
					"limit":           intProp("Maximum number of releases to list", 1, 1000),
					"flags":           flagsProp,
				},
			},
		},
		{
			Name:        "gh_release_view",
			Description: "View a release.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"repository_path": repoProp,
					"tag":             stringProp("Release tag"),
					"repo":            stringProp("Repository in OWNER/REPO format (optional)"),
					"web":             stringProp("Open release in browser (true/false)"),
					"flags":           flagsProp,
				},
				Required: []string{"tag"},
			},
		},
		{
			Name:        "gh_release_create",
			Description: "Create a new release.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"repository_path": repoProp,
					"tag":             stringProp("Release tag"),
					"title":           stringProp("Release title"),
					"notes":           stringProp("Release notes"),
					"draft":           stringProp("Create as draft (true/false)"),
					"prerelease":      stringProp("Mark as prerelease (true/false)"),
					"repo":            stringProp("Repository in OWNER/REPO format (optional)"),
					"flags":           flagsProp,
				},
				Required: []string{"tag"},
			},
		},
		{
			Name:        "gh_release_download",
			Description: "Download release assets.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"repository_path": repoProp,
					"tag":             stringProp("Release tag"),
					"pattern":         stringProp("Asset name pattern to download"),
					"repo":            stringProp("Repository in OWNER/REPO format (optional)"),
					"flags":           flagsProp,
				},
				Required: []string{"tag"},
			},
		},

		// --- Gist operations ---
		{
			Name:        "gh_gist_list",
			Description: "List gists.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"limit":  intProp("Maximum number of gists to list", 1, 1000),
					"public": stringProp("Show only public gists (true/false)"),
					"flags":  flagsProp,
				},
			},
		},
		{
			Name:        "gh_gist_view",
			Description: "View a gist.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"gist_id": stringProp("Gist ID or URL"),
					"raw":     stringProp("Print raw contents (true/false)"),
					"flags":   flagsProp,
				},
				Required: []string{"gist_id"},
			},
		},
		{
			Name:        "gh_gist_create",
			Description: "Create a new gist.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"files":       stringArrayProp("Files to include in gist"),
					"description": stringProp("Gist description"),
					"public":      stringProp("Make gist public (true/false)"),
					"flags":       flagsProp,
				},
				Required: []string{"files"},
			},
		},

		// --- Auth operations ---
		{
			Name:        "gh_auth_status",
			Description: "View authentication status.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"hostname": stringProp("Check authentication for specific hostname"),
					"flags":    flagsProp,
				},
			},
		},
		{
			Name:        "gh_auth_login",
			Description: "Authenticate with GitHub.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"hostname": stringProp("GitHub hostname (default: github.com)"),
					"web":      stringProp("Authenticate via web browser (true/false)"),
					"flags":    flagsProp,
				},
			},
		},

		// --- General operations ---
		{
			Name:        "gh_search_repos",
			Description: "Search for repositories.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"query": stringProp("Search query"),
					"limit": intProp("Maximum number of results", 1, 1000),
					"flags": flagsProp,
				},
				Required: []string{"query"},
			},
		},
		{
			Name:        "gh_search_issues",
			Description: "Search for issues and pull requests.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"query": stringProp("Search query"),
					"limit": intProp("Maximum number of results", 1, 1000),
					"flags": flagsProp,
				},
				Required: []string{"query"},
			},
		},
		{
			Name:        "gh_api",
			Description: "Make an authenticated GitHub API request.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"endpoint": stringProp("API endpoint (e.g., /repos/OWNER/REPO)"),
					"method":   stringProp("HTTP method (GET, POST, PUT, DELETE, PATCH)"),
					"field":    stringArrayProp("Add a parameter in key=value format"),
					"flags":    flagsProp,
				},
				Required: []string{"endpoint"},
			},
		},
	}

	s.sendResponse(req.ID, ListToolsResult{Tools: tools})
}

// ---------- Tool dispatch ----------

func (s *MCPServer) handleCallTool(req JSONRPCRequest) {
	var params CallToolParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		logger.Printf("Invalid params: %v\n", err)
		s.sendError(req.ID, -32602, "Invalid params", err.Error())
		return
	}

	logger.Printf("Calling tool: %s\n", params.Name)
	args := params.Arguments

	switch params.Name {
	// Repository
	case "gh_repo_view":
		s.ghRepoView(req.ID, args)
	case "gh_repo_clone":
		s.ghRepoClone(req.ID, args)
	case "gh_repo_create":
		s.ghRepoCreate(req.ID, args)
	case "gh_repo_fork":
		s.ghRepoFork(req.ID, args)
	case "gh_repo_list":
		s.ghRepoList(req.ID, args)

	// Issues
	case "gh_issue_list":
		s.ghIssueList(req.ID, args)
	case "gh_issue_view":
		s.ghIssueView(req.ID, args)
	case "gh_issue_create":
		s.ghIssueCreate(req.ID, args)
	case "gh_issue_close":
		s.ghIssueClose(req.ID, args)
	case "gh_issue_reopen":
		s.ghIssueReopen(req.ID, args)

	// Pull Requests
	case "gh_pr_list":
		s.ghPRList(req.ID, args)
	case "gh_pr_view":
		s.ghPRView(req.ID, args)
	case "gh_pr_create":
		s.ghPRCreate(req.ID, args)
	case "gh_pr_checkout":
		s.ghPRCheckout(req.ID, args)
	case "gh_pr_merge":
		s.ghPRMerge(req.ID, args)
	case "gh_pr_close":
		s.ghPRClose(req.ID, args)
	case "gh_pr_review":
		s.ghPRReview(req.ID, args)
	case "gh_pr_diff":
		s.ghPRDiff(req.ID, args)

	// Workflows
	case "gh_run_list":
		s.ghRunList(req.ID, args)
	case "gh_run_view":
		s.ghRunView(req.ID, args)
	case "gh_run_rerun":
		s.ghRunRerun(req.ID, args)
	case "gh_workflow_list":
		s.ghWorkflowList(req.ID, args)
	case "gh_workflow_run":
		s.ghWorkflowRun(req.ID, args)

	// Releases
	case "gh_release_list":
		s.ghReleaseList(req.ID, args)
	case "gh_release_view":
		s.ghReleaseView(req.ID, args)
	case "gh_release_create":
		s.ghReleaseCreate(req.ID, args)
	case "gh_release_download":
		s.ghReleaseDownload(req.ID, args)

	// Gists
	case "gh_gist_list":
		s.ghGistList(req.ID, args)
	case "gh_gist_view":
		s.ghGistView(req.ID, args)
	case "gh_gist_create":
		s.ghGistCreate(req.ID, args)

	// Auth
	case "gh_auth_status":
		s.ghAuthStatus(req.ID, args)
	case "gh_auth_login":
		s.ghAuthLogin(req.ID, args)

	// Search
	case "gh_search_repos":
		s.ghSearchRepos(req.ID, args)
	case "gh_search_issues":
		s.ghSearchIssues(req.ID, args)

	// API
	case "gh_api":
		s.ghAPI(req.ID, args)

	default:
		s.sendToolError(req.ID, fmt.Sprintf("Unknown tool: %s", params.Name))
	}
}

// ---------- Repository handlers ----------

func (s *MCPServer) ghRepoView(id interface{}, args map[string]interface{}) {
	cmdArgs := []string{"repo", "view"}
	
	if repo, ok := args["repo"].(string); ok && repo != "" {
		cmdArgs = append(cmdArgs, repo)
	}
	
	if web, ok := args["web"].(string); ok && web == "true" {
		cmdArgs = append(cmdArgs, "--web")
	}
	
	flags, _ := getFlags(args)
	cmdArgs = append(cmdArgs, flags...)
	
	cwd := getRepoPath(args)
	s.runGh(id, cwd, cmdArgs)
}

func (s *MCPServer) ghRepoClone(id interface{}, args map[string]interface{}) {
	repo, _ := args["repo"].(string)
	if repo == "" {
		s.sendToolError(id, "repo is required")
		return
	}
	
	cmdArgs := []string{"repo", "clone", repo}
	
	if path, ok := args["path"].(string); ok && path != "" {
		cmdArgs = append(cmdArgs, path)
	}
	
	flags, _ := getFlags(args)
	cmdArgs = append(cmdArgs, flags...)
	
	s.runGh(id, "", cmdArgs)
}

func (s *MCPServer) ghRepoCreate(id interface{}, args map[string]interface{}) {
	name, _ := args["name"].(string)
	if name == "" {
		s.sendToolError(id, "name is required")
		return
	}
	
	cmdArgs := []string{"repo", "create", name}
	
	if desc, ok := args["description"].(string); ok && desc != "" {
		cmdArgs = append(cmdArgs, "--description", desc)
	}
	
	if public, ok := args["public"].(string); ok && public == "true" {
		cmdArgs = append(cmdArgs, "--public")
	} else {
		cmdArgs = append(cmdArgs, "--private")
	}
	
	flags, _ := getFlags(args)
	cmdArgs = append(cmdArgs, flags...)
	
	s.runGh(id, "", cmdArgs)
}

func (s *MCPServer) ghRepoFork(id interface{}, args map[string]interface{}) {
	repo, _ := args["repo"].(string)
	if repo == "" {
		s.sendToolError(id, "repo is required")
		return
	}
	
	cmdArgs := []string{"repo", "fork", repo}
	
	if clone, ok := args["clone"].(string); ok && clone == "true" {
		cmdArgs = append(cmdArgs, "--clone")
	}
	
	flags, _ := getFlags(args)
	cmdArgs = append(cmdArgs, flags...)
	
	s.runGh(id, "", cmdArgs)
}

func (s *MCPServer) ghRepoList(id interface{}, args map[string]interface{}) {
	cmdArgs := []string{"repo", "list"}
	
	if owner, ok := args["owner"].(string); ok && owner != "" {
		cmdArgs = append(cmdArgs, owner)
	}
	
	if limit, ok := args["limit"].(float64); ok {
		cmdArgs = append(cmdArgs, "--limit", fmt.Sprintf("%d", int(limit)))
	}
	
	flags, _ := getFlags(args)
	cmdArgs = append(cmdArgs, flags...)
	
	s.runGh(id, "", cmdArgs)
}

// ---------- Issue handlers ----------

func (s *MCPServer) ghIssueList(id interface{}, args map[string]interface{}) {
	cmdArgs := []string{"issue", "list"}
	
	if repo, ok := args["repo"].(string); ok && repo != "" {
		cmdArgs = append(cmdArgs, "--repo", repo)
	}
	
	if state, ok := args["state"].(string); ok && state != "" {
		cmdArgs = append(cmdArgs, "--state", state)
	}
	
	if assignee, ok := args["assignee"].(string); ok && assignee != "" {
		cmdArgs = append(cmdArgs, "--assignee", assignee)
	}
	
	if label, ok := args["label"].(string); ok && label != "" {
		cmdArgs = append(cmdArgs, "--label", label)
	}
	
	if limit, ok := args["limit"].(float64); ok {
		cmdArgs = append(cmdArgs, "--limit", fmt.Sprintf("%d", int(limit)))
	}
	
	flags, _ := getFlags(args)
	cmdArgs = append(cmdArgs, flags...)
	
	cwd := getRepoPath(args)
	s.runGh(id, cwd, cmdArgs)
}

func (s *MCPServer) ghIssueView(id interface{}, args map[string]interface{}) {
	number, _ := args["number"].(string)
	if number == "" {
		s.sendToolError(id, "number is required")
		return
	}
	
	cmdArgs := []string{"issue", "view", number}
	
	if repo, ok := args["repo"].(string); ok && repo != "" {
		cmdArgs = append(cmdArgs, "--repo", repo)
	}
	
	if web, ok := args["web"].(string); ok && web == "true" {
		cmdArgs = append(cmdArgs, "--web")
	}
	
	flags, _ := getFlags(args)
	cmdArgs = append(cmdArgs, flags...)
	
	cwd := getRepoPath(args)
	s.runGh(id, cwd, cmdArgs)
}

func (s *MCPServer) ghIssueCreate(id interface{}, args map[string]interface{}) {
	title, _ := args["title"].(string)
	if title == "" {
		s.sendToolError(id, "title is required")
		return
	}
	
	cmdArgs := []string{"issue", "create", "--title", title}
	
	if body, ok := args["body"].(string); ok && body != "" {
		cmdArgs = append(cmdArgs, "--body", body)
	}
	
	if assignee, ok := args["assignee"].(string); ok && assignee != "" {
		cmdArgs = append(cmdArgs, "--assignee", assignee)
	}
	
	if labels := getStringArray(args, "label"); len(labels) > 0 {
		for _, label := range labels {
			cmdArgs = append(cmdArgs, "--label", label)
		}
	}
	
	if repo, ok := args["repo"].(string); ok && repo != "" {
		cmdArgs = append(cmdArgs, "--repo", repo)
	}
	
	flags, _ := getFlags(args)
	cmdArgs = append(cmdArgs, flags...)
	
	cwd := getRepoPath(args)
	s.runGh(id, cwd, cmdArgs)
}

func (s *MCPServer) ghIssueClose(id interface{}, args map[string]interface{}) {
	number, _ := args["number"].(string)
	if number == "" {
		s.sendToolError(id, "number is required")
		return
	}
	
	cmdArgs := []string{"issue", "close", number}
	
	if repo, ok := args["repo"].(string); ok && repo != "" {
		cmdArgs = append(cmdArgs, "--repo", repo)
	}
	
	flags, _ := getFlags(args)
	cmdArgs = append(cmdArgs, flags...)
	
	cwd := getRepoPath(args)
	s.runGh(id, cwd, cmdArgs)
}

func (s *MCPServer) ghIssueReopen(id interface{}, args map[string]interface{}) {
	number, _ := args["number"].(string)
	if number == "" {
		s.sendToolError(id, "number is required")
		return
	}
	
	cmdArgs := []string{"issue", "reopen", number}
	
	if repo, ok := args["repo"].(string); ok && repo != "" {
		cmdArgs = append(cmdArgs, "--repo", repo)
	}
	
	flags, _ := getFlags(args)
	cmdArgs = append(cmdArgs, flags...)
	
	cwd := getRepoPath(args)
	s.runGh(id, cwd, cmdArgs)
}

// ---------- Pull Request handlers ----------

func (s *MCPServer) ghPRList(id interface{}, args map[string]interface{}) {
	cmdArgs := []string{"pr", "list"}
	
	if repo, ok := args["repo"].(string); ok && repo != "" {
		cmdArgs = append(cmdArgs, "--repo", repo)
	}
	
	if state, ok := args["state"].(string); ok && state != "" {
		cmdArgs = append(cmdArgs, "--state", state)
	}
	
	if author, ok := args["author"].(string); ok && author != "" {
		cmdArgs = append(cmdArgs, "--author", author)
	}
	
	if assignee, ok := args["assignee"].(string); ok && assignee != "" {
		cmdArgs = append(cmdArgs, "--assignee", assignee)
	}
	
	if label, ok := args["label"].(string); ok && label != "" {
		cmdArgs = append(cmdArgs, "--label", label)
	}
	
	if limit, ok := args["limit"].(float64); ok {
		cmdArgs = append(cmdArgs, "--limit", fmt.Sprintf("%d", int(limit)))
	}
	
	flags, _ := getFlags(args)
	cmdArgs = append(cmdArgs, flags...)
	
	cwd := getRepoPath(args)
	s.runGh(id, cwd, cmdArgs)
}

func (s *MCPServer) ghPRView(id interface{}, args map[string]interface{}) {
	number, _ := args["number"].(string)
	if number == "" {
		s.sendToolError(id, "number is required")
		return
	}
	
	cmdArgs := []string{"pr", "view", number}
	
	if repo, ok := args["repo"].(string); ok && repo != "" {
		cmdArgs = append(cmdArgs, "--repo", repo)
	}
	
	if web, ok := args["web"].(string); ok && web == "true" {
		cmdArgs = append(cmdArgs, "--web")
	}
	
	flags, _ := getFlags(args)
	cmdArgs = append(cmdArgs, flags...)
	
	cwd := getRepoPath(args)
	s.runGh(id, cwd, cmdArgs)
}

func (s *MCPServer) ghPRCreate(id interface{}, args map[string]interface{}) {
	title, _ := args["title"].(string)
	if title == "" {
		s.sendToolError(id, "title is required")
		return
	}
	
	cmdArgs := []string{"pr", "create", "--title", title}
	
	if body, ok := args["body"].(string); ok && body != "" {
		cmdArgs = append(cmdArgs, "--body", body)
	}
	
	if base, ok := args["base"].(string); ok && base != "" {
		cmdArgs = append(cmdArgs, "--base", base)
	}
	
	if head, ok := args["head"].(string); ok && head != "" {
		cmdArgs = append(cmdArgs, "--head", head)
	}
	
	if draft, ok := args["draft"].(string); ok && draft == "true" {
		cmdArgs = append(cmdArgs, "--draft")
	}
	
	if assignee, ok := args["assignee"].(string); ok && assignee != "" {
		cmdArgs = append(cmdArgs, "--assignee", assignee)
	}
	
	if labels := getStringArray(args, "label"); len(labels) > 0 {
		for _, label := range labels {
			cmdArgs = append(cmdArgs, "--label", label)
		}
	}
	
	if repo, ok := args["repo"].(string); ok && repo != "" {
		cmdArgs = append(cmdArgs, "--repo", repo)
	}
	
	flags, _ := getFlags(args)
	cmdArgs = append(cmdArgs, flags...)
	
	cwd := getRepoPath(args)
	s.runGh(id, cwd, cmdArgs)
}

func (s *MCPServer) ghPRCheckout(id interface{}, args map[string]interface{}) {
	number, _ := args["number"].(string)
	if number == "" {
		s.sendToolError(id, "number is required")
		return
	}
	
	cmdArgs := []string{"pr", "checkout", number}
	
	if repo, ok := args["repo"].(string); ok && repo != "" {
		cmdArgs = append(cmdArgs, "--repo", repo)
	}
	
	flags, _ := getFlags(args)
	cmdArgs = append(cmdArgs, flags...)
	
	cwd := getRepoPath(args)
	s.runGh(id, cwd, cmdArgs)
}

func (s *MCPServer) ghPRMerge(id interface{}, args map[string]interface{}) {
	number, _ := args["number"].(string)
	if number == "" {
		s.sendToolError(id, "number is required")
		return
	}
	
	cmdArgs := []string{"pr", "merge", number}
	
	if method, ok := args["merge_method"].(string); ok && method != "" {
		switch method {
		case "merge":
			cmdArgs = append(cmdArgs, "--merge")
		case "squash":
			cmdArgs = append(cmdArgs, "--squash")
		case "rebase":
			cmdArgs = append(cmdArgs, "--rebase")
		}
	}
	
	if deleteBranch, ok := args["delete_branch"].(string); ok && deleteBranch == "true" {
		cmdArgs = append(cmdArgs, "--delete-branch")
	}
	
	if repo, ok := args["repo"].(string); ok && repo != "" {
		cmdArgs = append(cmdArgs, "--repo", repo)
	}
	
	flags, _ := getFlags(args)
	cmdArgs = append(cmdArgs, flags...)
	
	cwd := getRepoPath(args)
	s.runGh(id, cwd, cmdArgs)
}

func (s *MCPServer) ghPRClose(id interface{}, args map[string]interface{}) {
	number, _ := args["number"].(string)
	if number == "" {
		s.sendToolError(id, "number is required")
		return
	}
	
	cmdArgs := []string{"pr", "close", number}
	
	if deleteBranch, ok := args["delete_branch"].(string); ok && deleteBranch == "true" {
		cmdArgs = append(cmdArgs, "--delete-branch")
	}
	
	if repo, ok := args["repo"].(string); ok && repo != "" {
		cmdArgs = append(cmdArgs, "--repo", repo)
	}
	
	flags, _ := getFlags(args)
	cmdArgs = append(cmdArgs, flags...)
	
	cwd := getRepoPath(args)
	s.runGh(id, cwd, cmdArgs)
}

func (s *MCPServer) ghPRReview(id interface{}, args map[string]interface{}) {
	number, _ := args["number"].(string)
	if number == "" {
		s.sendToolError(id, "number is required")
		return
	}
	
	cmdArgs := []string{"pr", "review", number}
	
	if approve, ok := args["approve"].(string); ok && approve == "true" {
		cmdArgs = append(cmdArgs, "--approve")
	}
	
	if requestChanges, ok := args["request_changes"].(string); ok && requestChanges == "true" {
		cmdArgs = append(cmdArgs, "--request-changes")
	}
	
	if comment, ok := args["comment"].(string); ok && comment == "true" {
		cmdArgs = append(cmdArgs, "--comment")
	}
	
	if body, ok := args["body"].(string); ok && body != "" {
		cmdArgs = append(cmdArgs, "--body", body)
	}
	
	if repo, ok := args["repo"].(string); ok && repo != "" {
		cmdArgs = append(cmdArgs, "--repo", repo)
	}
	
	flags, _ := getFlags(args)
	cmdArgs = append(cmdArgs, flags...)
	
	cwd := getRepoPath(args)
	s.runGh(id, cwd, cmdArgs)
}

func (s *MCPServer) ghPRDiff(id interface{}, args map[string]interface{}) {
	number, _ := args["number"].(string)
	if number == "" {
		s.sendToolError(id, "number is required")
		return
	}
	
	cmdArgs := []string{"pr", "diff", number}
	
	if repo, ok := args["repo"].(string); ok && repo != "" {
		cmdArgs = append(cmdArgs, "--repo", repo)
	}
	
	flags, _ := getFlags(args)
	cmdArgs = append(cmdArgs, flags...)
	
	cwd := getRepoPath(args)
	s.runGh(id, cwd, cmdArgs)
}

// ---------- Workflow/Actions handlers ----------

func (s *MCPServer) ghRunList(id interface{}, args map[string]interface{}) {
	cmdArgs := []string{"run", "list"}
	
	if repo, ok := args["repo"].(string); ok && repo != "" {
		cmdArgs = append(cmdArgs, "--repo", repo)
	}
	
	if workflow, ok := args["workflow"].(string); ok && workflow != "" {
		cmdArgs = append(cmdArgs, "--workflow", workflow)
	}
	
	if limit, ok := args["limit"].(float64); ok {
		cmdArgs = append(cmdArgs, "--limit", fmt.Sprintf("%d", int(limit)))
	}
	
	flags, _ := getFlags(args)
	cmdArgs = append(cmdArgs, flags...)
	
	cwd := getRepoPath(args)
	s.runGh(id, cwd, cmdArgs)
}

func (s *MCPServer) ghRunView(id interface{}, args map[string]interface{}) {
	runID, _ := args["run_id"].(string)
	if runID == "" {
		s.sendToolError(id, "run_id is required")
		return
	}
	
	cmdArgs := []string{"run", "view", runID}
	
	if repo, ok := args["repo"].(string); ok && repo != "" {
		cmdArgs = append(cmdArgs, "--repo", repo)
	}
	
	if logView, ok := args["log"].(string); ok && logView == "true" {
		cmdArgs = append(cmdArgs, "--log")
	}
	
	flags, _ := getFlags(args)
	cmdArgs = append(cmdArgs, flags...)
	
	cwd := getRepoPath(args)
	s.runGh(id, cwd, cmdArgs)
}

func (s *MCPServer) ghRunRerun(id interface{}, args map[string]interface{}) {
	runID, _ := args["run_id"].(string)
	if runID == "" {
		s.sendToolError(id, "run_id is required")
		return
	}
	
	cmdArgs := []string{"run", "rerun", runID}
	
	if repo, ok := args["repo"].(string); ok && repo != "" {
		cmdArgs = append(cmdArgs, "--repo", repo)
	}
	
	flags, _ := getFlags(args)
	cmdArgs = append(cmdArgs, flags...)
	
	cwd := getRepoPath(args)
	s.runGh(id, cwd, cmdArgs)
}

func (s *MCPServer) ghWorkflowList(id interface{}, args map[string]interface{}) {
	cmdArgs := []string{"workflow", "list"}
	
	if repo, ok := args["repo"].(string); ok && repo != "" {
		cmdArgs = append(cmdArgs, "--repo", repo)
	}
	
	flags, _ := getFlags(args)
	cmdArgs = append(cmdArgs, flags...)
	
	cwd := getRepoPath(args)
	s.runGh(id, cwd, cmdArgs)
}

func (s *MCPServer) ghWorkflowRun(id interface{}, args map[string]interface{}) {
	workflow, _ := args["workflow"].(string)
	if workflow == "" {
		s.sendToolError(id, "workflow is required")
		return
	}
	
	cmdArgs := []string{"workflow", "run", workflow}
	
	if ref, ok := args["ref"].(string); ok && ref != "" {
		cmdArgs = append(cmdArgs, "--ref", ref)
	}
	
	if repo, ok := args["repo"].(string); ok && repo != "" {
		cmdArgs = append(cmdArgs, "--repo", repo)
	}
	
	flags, _ := getFlags(args)
	cmdArgs = append(cmdArgs, flags...)
	
	cwd := getRepoPath(args)
	s.runGh(id, cwd, cmdArgs)
}

// ---------- Release handlers ----------

func (s *MCPServer) ghReleaseList(id interface{}, args map[string]interface{}) {
	cmdArgs := []string{"release", "list"}
	
	if repo, ok := args["repo"].(string); ok && repo != "" {
		cmdArgs = append(cmdArgs, "--repo", repo)
	}
	
	if limit, ok := args["limit"].(float64); ok {
		cmdArgs = append(cmdArgs, "--limit", fmt.Sprintf("%d", int(limit)))
	}
	
	flags, _ := getFlags(args)
	cmdArgs = append(cmdArgs, flags...)
	
	cwd := getRepoPath(args)
	s.runGh(id, cwd, cmdArgs)
}

func (s *MCPServer) ghReleaseView(id interface{}, args map[string]interface{}) {
	tag, _ := args["tag"].(string)
	if tag == "" {
		s.sendToolError(id, "tag is required")
		return
	}
	
	cmdArgs := []string{"release", "view", tag}
	
	if repo, ok := args["repo"].(string); ok && repo != "" {
		cmdArgs = append(cmdArgs, "--repo", repo)
	}
	
	if web, ok := args["web"].(string); ok && web == "true" {
		cmdArgs = append(cmdArgs, "--web")
	}
	
	flags, _ := getFlags(args)
	cmdArgs = append(cmdArgs, flags...)
	
	cwd := getRepoPath(args)
	s.runGh(id, cwd, cmdArgs)
}

func (s *MCPServer) ghReleaseCreate(id interface{}, args map[string]interface{}) {
	tag, _ := args["tag"].(string)
	if tag == "" {
		s.sendToolError(id, "tag is required")
		return
	}
	
	cmdArgs := []string{"release", "create", tag}
	
	if title, ok := args["title"].(string); ok && title != "" {
		cmdArgs = append(cmdArgs, "--title", title)
	}
	
	if notes, ok := args["notes"].(string); ok && notes != "" {
		cmdArgs = append(cmdArgs, "--notes", notes)
	}
	
	if draft, ok := args["draft"].(string); ok && draft == "true" {
		cmdArgs = append(cmdArgs, "--draft")
	}
	
	if prerelease, ok := args["prerelease"].(string); ok && prerelease == "true" {
		cmdArgs = append(cmdArgs, "--prerelease")
	}
	
	if repo, ok := args["repo"].(string); ok && repo != "" {
		cmdArgs = append(cmdArgs, "--repo", repo)
	}
	
	flags, _ := getFlags(args)
	cmdArgs = append(cmdArgs, flags...)
	
	cwd := getRepoPath(args)
	s.runGh(id, cwd, cmdArgs)
}

func (s *MCPServer) ghReleaseDownload(id interface{}, args map[string]interface{}) {
	tag, _ := args["tag"].(string)
	if tag == "" {
		s.sendToolError(id, "tag is required")
		return
	}
	
	cmdArgs := []string{"release", "download", tag}
	
	if pattern, ok := args["pattern"].(string); ok && pattern != "" {
		cmdArgs = append(cmdArgs, "--pattern", pattern)
	}
	
	if repo, ok := args["repo"].(string); ok && repo != "" {
		cmdArgs = append(cmdArgs, "--repo", repo)
	}
	
	flags, _ := getFlags(args)
	cmdArgs = append(cmdArgs, flags...)
	
	cwd := getRepoPath(args)
	s.runGh(id, cwd, cmdArgs)
}

// ---------- Gist handlers ----------

func (s *MCPServer) ghGistList(id interface{}, args map[string]interface{}) {
	cmdArgs := []string{"gist", "list"}
	
	if limit, ok := args["limit"].(float64); ok {
		cmdArgs = append(cmdArgs, "--limit", fmt.Sprintf("%d", int(limit)))
	}
	
	if public, ok := args["public"].(string); ok && public == "true" {
		cmdArgs = append(cmdArgs, "--public")
	}
	
	flags, _ := getFlags(args)
	cmdArgs = append(cmdArgs, flags...)
	
	s.runGh(id, "", cmdArgs)
}

func (s *MCPServer) ghGistView(id interface{}, args map[string]interface{}) {
	gistID, _ := args["gist_id"].(string)
	if gistID == "" {
		s.sendToolError(id, "gist_id is required")
		return
	}
	
	cmdArgs := []string{"gist", "view", gistID}
	
	if raw, ok := args["raw"].(string); ok && raw == "true" {
		cmdArgs = append(cmdArgs, "--raw")
	}
	
	flags, _ := getFlags(args)
	cmdArgs = append(cmdArgs, flags...)
	
	s.runGh(id, "", cmdArgs)
}

func (s *MCPServer) ghGistCreate(id interface{}, args map[string]interface{}) {
	files := getStringArray(args, "files")
	if len(files) == 0 {
		s.sendToolError(id, "files is required")
		return
	}
	
	cmdArgs := []string{"gist", "create"}
	cmdArgs = append(cmdArgs, files...)
	
	if desc, ok := args["description"].(string); ok && desc != "" {
		cmdArgs = append(cmdArgs, "--desc", desc)
	}
	
	if public, ok := args["public"].(string); ok && public == "true" {
		cmdArgs = append(cmdArgs, "--public")
	}
	
	flags, _ := getFlags(args)
	cmdArgs = append(cmdArgs, flags...)
	
	s.runGh(id, "", cmdArgs)
}

// ---------- Auth handlers ----------

func (s *MCPServer) ghAuthStatus(id interface{}, args map[string]interface{}) {
	cmdArgs := []string{"auth", "status"}
	
	if hostname, ok := args["hostname"].(string); ok && hostname != "" {
		cmdArgs = append(cmdArgs, "--hostname", hostname)
	}
	
	flags, _ := getFlags(args)
	cmdArgs = append(cmdArgs, flags...)
	
	s.runGh(id, "", cmdArgs)
}

func (s *MCPServer) ghAuthLogin(id interface{}, args map[string]interface{}) {
	cmdArgs := []string{"auth", "login"}
	
	if hostname, ok := args["hostname"].(string); ok && hostname != "" {
		cmdArgs = append(cmdArgs, "--hostname", hostname)
	}
	
	if web, ok := args["web"].(string); ok && web == "true" {
		cmdArgs = append(cmdArgs, "--web")
	}
	
	flags, _ := getFlags(args)
	cmdArgs = append(cmdArgs, flags...)
	
	s.runGh(id, "", cmdArgs)
}

// ---------- Search handlers ----------

func (s *MCPServer) ghSearchRepos(id interface{}, args map[string]interface{}) {
	query, _ := args["query"].(string)
	if query == "" {
		s.sendToolError(id, "query is required")
		return
	}
	
	cmdArgs := []string{"search", "repos", query}
	
	if limit, ok := args["limit"].(float64); ok {
		cmdArgs = append(cmdArgs, "--limit", fmt.Sprintf("%d", int(limit)))
	}
	
	flags, _ := getFlags(args)
	cmdArgs = append(cmdArgs, flags...)
	
	s.runGh(id, "", cmdArgs)
}

func (s *MCPServer) ghSearchIssues(id interface{}, args map[string]interface{}) {
	query, _ := args["query"].(string)
	if query == "" {
		s.sendToolError(id, "query is required")
		return
	}
	
	cmdArgs := []string{"search", "issues", query}
	
	if limit, ok := args["limit"].(float64); ok {
		cmdArgs = append(cmdArgs, "--limit", fmt.Sprintf("%d", int(limit)))
	}
	
	flags, _ := getFlags(args)
	cmdArgs = append(cmdArgs, flags...)
	
	s.runGh(id, "", cmdArgs)
}

// ---------- API handler ----------

func (s *MCPServer) ghAPI(id interface{}, args map[string]interface{}) {
	endpoint, _ := args["endpoint"].(string)
	if endpoint == "" {
		s.sendToolError(id, "endpoint is required")
		return
	}
	
	cmdArgs := []string{"api", endpoint}
	
	if method, ok := args["method"].(string); ok && method != "" {
		cmdArgs = append(cmdArgs, "--method", method)
	}
	
	if fields := getStringArray(args, "field"); len(fields) > 0 {
		for _, field := range fields {
			cmdArgs = append(cmdArgs, "--field", field)
		}
	}
	
	flags, _ := getFlags(args)
	cmdArgs = append(cmdArgs, flags...)
	
	s.runGh(id, "", cmdArgs)
}

// ---------- GitHub CLI execution ----------

func (s *MCPServer) runGh(id interface{}, cwd string, ghArgs []string) {
	cmd := exec.Command("gh", ghArgs...)
	if cwd != "" {
		if err := validateRepoPath(cwd); err != nil {
			s.sendToolError(id, err.Error())
			return
		}
		cmd.Dir = cwd
	}

	commandStr := "gh " + strings.Join(ghArgs, " ")
	logger.Printf("Executing: %s (cwd: %s)\n", commandStr, cwd)

	stdout, err := cmd.Output()
	result := GhResult{
		Command: commandStr,
		Success: err == nil,
		Stdout:  strings.TrimSpace(string(stdout)),
	}

	if err != nil {
		logger.Printf("gh command failed: %v\n", err)
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.Stderr = strings.TrimSpace(string(exitErr.Stderr))
			logger.Printf("gh stderr: %s\n", result.Stderr)
		}
		result.Error = err.Error()
	} else {
		logger.Printf("gh command succeeded, stdout length: %d bytes\n", len(result.Stdout))
	}

	data, _ := json.MarshalIndent(result, "", "  ")
	s.sendResponse(id, ToolResult{
		Content: []ContentItem{{Type: "text", Text: string(data)}},
		IsError: !result.Success,
	})
}

// ---------- Helpers ----------

func getRepoPath(args map[string]interface{}) string {
	if p, ok := args["repository_path"].(string); ok && p != "" {
		return p
	}
	return ""
}

// allowedRepoPaths restricts which directories gh operations can target.
// Defaults to $HOME. Override via HUNTER3_GH_ALLOWED_PATHS (comma-separated).
var allowedRepoPaths []string

func initAllowedPaths() {
	if envPaths := os.Getenv("HUNTER3_GH_ALLOWED_PATHS"); envPaths != "" {
		for _, p := range strings.Split(envPaths, ",") {
			p = strings.TrimSpace(p)
			if abs, err := filepath.Abs(p); err == nil {
				allowedRepoPaths = append(allowedRepoPaths, filepath.Clean(abs))
			}
		}
	}
	if len(allowedRepoPaths) == 0 {
		if home := os.Getenv("HOME"); home != "" {
			allowedRepoPaths = []string{filepath.Clean(home)}
		}
	}
}

func validateRepoPath(repoPath string) error {
	if len(allowedRepoPaths) == 0 {
		return nil
	}

	absPath, err := filepath.Abs(repoPath)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}
	normalized := filepath.Clean(absPath)

	for _, allowed := range allowedRepoPaths {
		if normalized == allowed || strings.HasPrefix(normalized, allowed+string(filepath.Separator)) {
			return nil
		}
	}
	return fmt.Errorf("path %q is outside allowed directories", repoPath)
}

func getFlags(args map[string]interface{}) ([]string, error) {
	return getStringArray(args, "flags"), nil
}

func getStringArray(args map[string]interface{}, key string) []string {
	val, ok := args[key]
	if !ok {
		return nil
	}

	arr, ok := val.([]interface{})
	if !ok {
		return nil
	}

	result := make([]string, 0, len(arr))
	for _, v := range arr {
		if s, ok := v.(string); ok {
			result = append(result, s)
		}
	}
	return result
}

// ---------- JSON-RPC responses ----------

func (s *MCPServer) sendResponse(id interface{}, result interface{}) {
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
	data, err := json.Marshal(resp)
	if err != nil {
		logger.Printf("Error marshaling response: %v\n", err)
		fmt.Fprintf(os.Stderr, "Error marshaling response: %v\n", err)
		return
	}
	fmt.Println(string(data))
	logger.Printf("Sent response for request ID: %v\n", id)
}

func (s *MCPServer) sendError(id interface{}, code int, message string, data interface{}) {
	logger.Printf("Sending error response: code=%d, message=%s\n", code, message)
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &RPCError{Code: code, Message: message, Data: data},
	}
	jsonData, err := json.Marshal(resp)
	if err != nil {
		logger.Printf("Error marshaling error response: %v\n", err)
		fmt.Fprintf(os.Stderr, "Error marshaling error response: %v\n", err)
		return
	}
	fmt.Println(string(jsonData))
}

func (s *MCPServer) sendToolError(id interface{}, msg string) {
	s.sendResponse(id, ToolResult{
		Content: []ContentItem{{Type: "text", Text: msg}},
		IsError: true,
	})
}
