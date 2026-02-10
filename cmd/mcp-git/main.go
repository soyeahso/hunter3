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

// GitResult is returned from executeGitCommand as JSON.
type GitResult struct {
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
	logFile := filepath.Join(logsDir, "mcp-git.log")
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open log file: %v\n", err)
		return
	}

	// Create logger that writes to both file and stderr
	logger = log.New(io.MultiWriter(f, os.Stderr), "[mcp-git] ", log.LstdFlags)
	logger.Println("MCP Git server starting...")
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
		ServerInfo:      ServerInfo{Name: "mcp-git", Version: "1.0.0"},
	})
}

// ---------- Tool definitions ----------

func (s *MCPServer) handleListTools(req JSONRPCRequest) {
	logger.Println("Handling list tools request")
	flagsProp := stringArrayProp("Additional flags passed directly to the git command")

	// Common property sets
	repoProp := stringProp("Path to the git repository (working directory for the command)")

	tools := []Tool{
		// --- Porcelain: getting info ---
		{
			Name:        "git_status",
			Description: "Show the working tree status. Supports flags like --short, --branch, --porcelain, etc.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"repository_path": repoProp,
					"flags":           flagsProp,
				},
				Required: []string{"repository_path"},
			},
		},
		{
			Name:        "git_log",
			Description: "Show commit logs. Supports flags like --oneline, --graph, --all, -n, --author, --since, --format, etc.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"repository_path": repoProp,
					"flags":           flagsProp,
				},
				Required: []string{"repository_path"},
			},
		},
		{
			Name:        "git_diff",
			Description: "Show changes between commits, commit and working tree, etc. Supports flags like --staged, --cached, --stat, --name-only, etc.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"repository_path": repoProp,
					"target":          stringProp("Commit, branch, or path to diff against (e.g. 'HEAD~1', 'main', 'file.go')"),
					"flags":           flagsProp,
				},
				Required: []string{"repository_path"},
			},
		},
		{
			Name:        "git_show",
			Description: "Show various types of objects (commits, tags, etc.). Supports flags like --stat, --format, etc.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"repository_path": repoProp,
					"object":          stringProp("Object to show (commit SHA, tag, HEAD, etc.). Defaults to HEAD."),
					"flags":           flagsProp,
				},
				Required: []string{"repository_path"},
			},
		},
		{
			Name:        "git_blame",
			Description: "Show what revision and author last modified each line of a file.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"repository_path": repoProp,
					"file":            stringProp("File to annotate"),
					"flags":           flagsProp,
				},
				Required: []string{"repository_path", "file"},
			},
		},

		// --- Porcelain: staging & committing ---
		{
			Name:        "git_add",
			Description: "Add file contents to the staging area. Supports patterns and flags like -A, --all, --force, --dry-run, etc.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"repository_path": repoProp,
					"paths":           stringArrayProp("File paths or patterns to add (e.g. [\".\", \"*.go\", \"src/\"])"),
					"flags":           flagsProp,
				},
				Required: []string{"repository_path"},
			},
		},
		{
			Name:        "git_commit",
			Description: "Record changes to the repository. Supports flags like --amend, --no-verify, --signoff, --allow-empty, etc.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"repository_path": repoProp,
					"message":         stringProp("Commit message"),
					"flags":           flagsProp,
				},
				Required: []string{"repository_path", "message"},
			},
		},
		{
			Name:        "git_reset",
			Description: "Reset current HEAD to the specified state. Supports --soft, --mixed, --hard, and paths.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"repository_path": repoProp,
					"target":          stringProp("Commit or reference to reset to (e.g. 'HEAD~1', commit SHA)"),
					"flags":           flagsProp,
				},
				Required: []string{"repository_path"},
			},
		},
		{
			Name:        "git_restore",
			Description: "Restore working tree files. Supports --staged, --source, --worktree, etc.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"repository_path": repoProp,
					"paths":           stringArrayProp("File paths to restore"),
					"flags":           flagsProp,
				},
				Required: []string{"repository_path"},
			},
		},
		{
			Name:        "git_rm",
			Description: "Remove files from the working tree and the index. Supports --cached, --force, -r, etc.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"repository_path": repoProp,
					"paths":           stringArrayProp("File paths to remove"),
					"flags":           flagsProp,
				},
				Required: []string{"repository_path", "paths"},
			},
		},
		{
			Name:        "git_mv",
			Description: "Move or rename a file, directory, or symlink.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"repository_path": repoProp,
					"source":          stringProp("Source path"),
					"destination":     stringProp("Destination path"),
					"flags":           flagsProp,
				},
				Required: []string{"repository_path", "source", "destination"},
			},
		},

		// --- Branching & merging ---
		{
			Name:        "git_branch",
			Description: "List, create, or delete branches. Supports flags like -d, -D, -m, --all, -r, etc.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"repository_path": repoProp,
					"branch_name":     stringProp("Branch name (omit to list branches)"),
					"flags":           flagsProp,
				},
				Required: []string{"repository_path"},
			},
		},
		{
			Name:        "git_checkout",
			Description: "Switch branches or restore working tree files. Supports flags like -b, -B, --track, etc.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"repository_path": repoProp,
					"target":          stringProp("Branch name, commit, tag, or file path to checkout"),
					"flags":           flagsProp,
				},
				Required: []string{"repository_path"},
			},
		},
		{
			Name:        "git_switch",
			Description: "Switch branches. Supports flags like -c (create), -d (detach), etc.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"repository_path": repoProp,
					"branch":          stringProp("Branch name to switch to"),
					"flags":           flagsProp,
				},
				Required: []string{"repository_path"},
			},
		},
		{
			Name:        "git_merge",
			Description: "Join two or more development histories together. Supports flags like --no-ff, --squash, --abort, etc.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"repository_path": repoProp,
					"branch":          stringProp("Branch to merge into current branch"),
					"flags":           flagsProp,
				},
				Required: []string{"repository_path"},
			},
		},
		{
			Name:        "git_rebase",
			Description: "Reapply commits on top of another base tip. Supports flags like --onto, --abort, --continue, --skip, etc.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"repository_path": repoProp,
					"target":          stringProp("Branch or commit to rebase onto"),
					"flags":           flagsProp,
				},
				Required: []string{"repository_path"},
			},
		},
		{
			Name:        "git_cherry_pick",
			Description: "Apply the changes introduced by existing commits. Supports flags like --no-commit, --abort, --continue, etc.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"repository_path": repoProp,
					"commits":         stringArrayProp("Commit SHAs to cherry-pick"),
					"flags":           flagsProp,
				},
				Required: []string{"repository_path", "commits"},
			},
		},

		// --- Remote operations ---
		{
			Name:        "git_remote",
			Description: "Manage remote repositories. Subcommands: add, remove, rename, get-url, set-url, or omit to list.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"repository_path": repoProp,
					"subcommand":      stringProp("Remote subcommand (add, remove, rename, get-url, set-url, or omit to list)"),
					"name":            stringProp("Name of the remote (e.g. 'origin')"),
					"url":             stringProp("Remote URL (for add/set-url)"),
					"flags":           flagsProp,
				},
				Required: []string{"repository_path"},
			},
		},
		{
			Name:        "git_fetch",
			Description: "Download objects and refs from a remote repository. Supports flags like --all, --prune, --tags, etc.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"repository_path": repoProp,
					"remote":          stringPropDefault("Remote name", "origin"),
					"flags":           flagsProp,
				},
				Required: []string{"repository_path"},
			},
		},
		{
			Name:        "git_pull",
			Description: "Fetch from and integrate with another repository or branch. Supports flags like --rebase, --no-rebase, --ff-only, etc.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"repository_path": repoProp,
					"remote":          stringPropDefault("Remote name", "origin"),
					"branch":          stringProp("Branch to pull (omit to pull current tracking branch)"),
					"flags":           flagsProp,
				},
				Required: []string{"repository_path"},
			},
		},
		{
			Name:        "git_push",
			Description: "Update remote refs along with associated objects. Supports flags like --force, --tags, --set-upstream, --delete, etc.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"repository_path": repoProp,
					"remote":          stringPropDefault("Remote name", "origin"),
					"branch":          stringProp("Branch name to push (omit to push current branch)"),
					"flags":           flagsProp,
				},
				Required: []string{"repository_path"},
			},
		},
		{
			Name:        "git_clone",
			Description: "Clone a repository into a new directory.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"url":  stringProp("Repository URL to clone"),
					"path": stringProp("Local path to clone into (optional)"),
					"flags": flagsProp,
				},
				Required: []string{"url"},
			},
		},

		// --- Tags ---
		{
			Name:        "git_tag",
			Description: "Create, list, or delete tags. Supports flags like -a, -m, -d, -l, --sort, etc.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"repository_path": repoProp,
					"tag_name":        stringProp("Tag name (omit to list tags)"),
					"message":         stringProp("Tag message (for annotated tags with -a)"),
					"flags":           flagsProp,
				},
				Required: []string{"repository_path"},
			},
		},

		// --- Stash ---
		{
			Name:        "git_stash",
			Description: "Stash changes in a dirty working directory. Subcommands: push, pop, apply, list, drop, show, clear.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"repository_path": repoProp,
					"subcommand":      stringPropDefault("Stash subcommand (push, pop, apply, list, drop, show, clear)", "push"),
					"message":         stringProp("Stash message (for push)"),
					"flags":           flagsProp,
				},
				Required: []string{"repository_path"},
			},
		},

		// --- Working tree ---
		{
			Name:        "git_clean",
			Description: "Remove untracked files from the working tree. Supports flags like -f, -d, -n (dry-run), -x, etc.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"repository_path": repoProp,
					"flags":           flagsProp,
				},
				Required: []string{"repository_path"},
			},
		},

		// --- Repository setup ---
		{
			Name:        "git_init",
			Description: "Initialize a new Git repository. Supports flags like --bare, --initial-branch, etc.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"path":  stringProp("Path where to initialize the repository (defaults to current directory)"),
					"flags": flagsProp,
				},
			},
		},

		// --- Plumbing / info ---
		{
			Name:        "git_rev_parse",
			Description: "Parse revision or other git info. Useful for getting the current branch (--abbrev-ref HEAD), repo root (--show-toplevel), etc.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"repository_path": repoProp,
					"args":            stringArrayProp("Arguments to git rev-parse (e.g. ['--abbrev-ref', 'HEAD'])"),
					"flags":           flagsProp,
				},
				Required: []string{"repository_path", "args"},
			},
		},
		{
			Name:        "git_ls_files",
			Description: "Show information about files in the index and working tree. Supports flags like --modified, --deleted, --others, --ignored, etc.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"repository_path": repoProp,
					"flags":           flagsProp,
				},
				Required: []string{"repository_path"},
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
	case "git_status":
		s.gitSimple(req.ID, args, "status")
	case "git_log":
		s.gitSimple(req.ID, args, "log")
	case "git_diff":
		s.gitWithTarget(req.ID, args, "diff", "target")
	case "git_show":
		s.gitWithTarget(req.ID, args, "show", "object")
	case "git_blame":
		s.gitBlame(req.ID, args)
	case "git_add":
		s.gitWithPaths(req.ID, args, "add")
	case "git_commit":
		s.gitCommit(req.ID, args)
	case "git_reset":
		s.gitWithTarget(req.ID, args, "reset", "target")
	case "git_restore":
		s.gitWithPaths(req.ID, args, "restore")
	case "git_rm":
		s.gitWithPaths(req.ID, args, "rm")
	case "git_mv":
		s.gitMv(req.ID, args)
	case "git_branch":
		s.gitWithTarget(req.ID, args, "branch", "branch_name")
	case "git_checkout":
		s.gitWithTarget(req.ID, args, "checkout", "target")
	case "git_switch":
		s.gitWithTarget(req.ID, args, "switch", "branch")
	case "git_merge":
		s.gitWithTarget(req.ID, args, "merge", "branch")
	case "git_rebase":
		s.gitWithTarget(req.ID, args, "rebase", "target")
	case "git_cherry_pick":
		s.gitCherryPick(req.ID, args)
	case "git_remote":
		s.gitRemote(req.ID, args)
	case "git_fetch":
		s.gitRemoteOp(req.ID, args, "fetch")
	case "git_pull":
		s.gitPullPush(req.ID, args, "pull")
	case "git_push":
		s.gitPullPush(req.ID, args, "push")
	case "git_clone":
		s.gitClone(req.ID, args)
	case "git_tag":
		s.gitTag(req.ID, args)
	case "git_stash":
		s.gitStash(req.ID, args)
	case "git_clean":
		s.gitSimple(req.ID, args, "clean")
	case "git_init":
		s.gitInit(req.ID, args)
	case "git_rev_parse":
		s.gitRevParse(req.ID, args)
	case "git_ls_files":
		s.gitSimple(req.ID, args, "ls-files")
	default:
		s.sendToolError(req.ID, fmt.Sprintf("Unknown tool: %s", params.Name))
	}
}

// ---------- Tool handlers ----------

// gitSimple handles commands that just take repository_path + flags (status, log, clean, ls-files).
func (s *MCPServer) gitSimple(id interface{}, args map[string]interface{}, subcmd string) {
	repoPath, ok := getRepoPath(args)
	if !ok {
		s.sendToolError(id, "repository_path is required")
		return
	}
	if err := verifyRepo(repoPath); err != nil {
		s.sendToolError(id, err.Error())
		return
	}

	cmdArgs := []string{subcmd}
	flags, err := getFlags(args)
	if err != nil {
		s.sendToolError(id, err.Error())
		return
	}
	cmdArgs = append(cmdArgs, flags...)

	s.runGit(id, repoPath, cmdArgs)
}

// gitWithTarget handles commands with an optional positional argument (diff, show, branch, checkout, etc.).
func (s *MCPServer) gitWithTarget(id interface{}, args map[string]interface{}, subcmd, targetKey string) {
	repoPath, ok := getRepoPath(args)
	if !ok {
		s.sendToolError(id, "repository_path is required")
		return
	}
	if err := verifyRepo(repoPath); err != nil {
		s.sendToolError(id, err.Error())
		return
	}

	cmdArgs := []string{subcmd}
	flags, err := getFlags(args)
	if err != nil {
		s.sendToolError(id, err.Error())
		return
	}
	cmdArgs = append(cmdArgs, flags...)
	if target, ok := args[targetKey].(string); ok && target != "" {
		cmdArgs = append(cmdArgs, target)
	}

	s.runGit(id, repoPath, cmdArgs)
}

// gitWithPaths handles commands that take an array of paths (add, restore, rm).
func (s *MCPServer) gitWithPaths(id interface{}, args map[string]interface{}, subcmd string) {
	repoPath, ok := getRepoPath(args)
	if !ok {
		s.sendToolError(id, "repository_path is required")
		return
	}
	if err := verifyRepo(repoPath); err != nil {
		s.sendToolError(id, err.Error())
		return
	}

	cmdArgs := []string{subcmd}
	flags, err := getFlags(args)
	if err != nil {
		s.sendToolError(id, err.Error())
		return
	}
	cmdArgs = append(cmdArgs, flags...)

	paths := getStringArray(args, "paths")
	if len(paths) == 0 && subcmd == "add" {
		paths = []string{"."}
	}
	cmdArgs = append(cmdArgs, paths...)

	s.runGit(id, repoPath, cmdArgs)
}

// gitBlame handles git blame with a required file argument.
func (s *MCPServer) gitBlame(id interface{}, args map[string]interface{}) {
	repoPath, ok := getRepoPath(args)
	if !ok {
		s.sendToolError(id, "repository_path is required")
		return
	}
	if err := verifyRepo(repoPath); err != nil {
		s.sendToolError(id, err.Error())
		return
	}

	file, _ := args["file"].(string)
	if file == "" {
		s.sendToolError(id, "file is required")
		return
	}

	cmdArgs := []string{"blame"}
	flags, err := getFlags(args)
	if err != nil {
		s.sendToolError(id, err.Error())
		return
	}
	cmdArgs = append(cmdArgs, flags...)
	cmdArgs = append(cmdArgs, file)

	s.runGit(id, repoPath, cmdArgs)
}

// gitCommit handles git commit with a -m message.
func (s *MCPServer) gitCommit(id interface{}, args map[string]interface{}) {
	repoPath, ok := getRepoPath(args)
	if !ok {
		s.sendToolError(id, "repository_path is required")
		return
	}
	if err := verifyRepo(repoPath); err != nil {
		s.sendToolError(id, err.Error())
		return
	}

	message, _ := args["message"].(string)
	if message == "" {
		s.sendToolError(id, "message is required")
		return
	}

	cmdArgs := []string{"commit"}
	flags, err := getFlags(args)
	if err != nil {
		s.sendToolError(id, err.Error())
		return
	}
	cmdArgs = append(cmdArgs, flags...)
	cmdArgs = append(cmdArgs, "-m", message)

	s.runGit(id, repoPath, cmdArgs)
}

// gitMv handles git mv with source and destination.
func (s *MCPServer) gitMv(id interface{}, args map[string]interface{}) {
	repoPath, ok := getRepoPath(args)
	if !ok {
		s.sendToolError(id, "repository_path is required")
		return
	}
	if err := verifyRepo(repoPath); err != nil {
		s.sendToolError(id, err.Error())
		return
	}

	source, _ := args["source"].(string)
	dest, _ := args["destination"].(string)
	if source == "" || dest == "" {
		s.sendToolError(id, "source and destination are required")
		return
	}

	cmdArgs := []string{"mv"}
	flags, err := getFlags(args)
	if err != nil {
		s.sendToolError(id, err.Error())
		return
	}
	cmdArgs = append(cmdArgs, flags...)
	cmdArgs = append(cmdArgs, source, dest)

	s.runGit(id, repoPath, cmdArgs)
}

// gitCherryPick handles git cherry-pick with commit SHAs.
func (s *MCPServer) gitCherryPick(id interface{}, args map[string]interface{}) {
	repoPath, ok := getRepoPath(args)
	if !ok {
		s.sendToolError(id, "repository_path is required")
		return
	}
	if err := verifyRepo(repoPath); err != nil {
		s.sendToolError(id, err.Error())
		return
	}

	commits := getStringArray(args, "commits")
	if len(commits) == 0 {
		s.sendToolError(id, "commits is required")
		return
	}

	cmdArgs := []string{"cherry-pick"}
	flags, err := getFlags(args)
	if err != nil {
		s.sendToolError(id, err.Error())
		return
	}
	cmdArgs = append(cmdArgs, flags...)
	cmdArgs = append(cmdArgs, commits...)

	s.runGit(id, repoPath, cmdArgs)
}

// gitRemote handles the git remote subcommand.
func (s *MCPServer) gitRemote(id interface{}, args map[string]interface{}) {
	repoPath, ok := getRepoPath(args)
	if !ok {
		s.sendToolError(id, "repository_path is required")
		return
	}
	if err := verifyRepo(repoPath); err != nil {
		s.sendToolError(id, err.Error())
		return
	}

	cmdArgs := []string{"remote"}

	if sub, ok := args["subcommand"].(string); ok && sub != "" {
		cmdArgs = append(cmdArgs, sub)
		if name, ok := args["name"].(string); ok && name != "" {
			cmdArgs = append(cmdArgs, name)
		}
		if u, ok := args["url"].(string); ok && u != "" {
			cmdArgs = append(cmdArgs, u)
		}
	}

	flags, err := getFlags(args)
	if err != nil {
		s.sendToolError(id, err.Error())
		return
	}
	cmdArgs = append(cmdArgs, flags...)
	s.runGit(id, repoPath, cmdArgs)
}

// gitRemoteOp handles git fetch (remote + flags only).
func (s *MCPServer) gitRemoteOp(id interface{}, args map[string]interface{}, subcmd string) {
	repoPath, ok := getRepoPath(args)
	if !ok {
		s.sendToolError(id, "repository_path is required")
		return
	}
	if err := verifyRepo(repoPath); err != nil {
		s.sendToolError(id, err.Error())
		return
	}

	cmdArgs := []string{subcmd}
	flags, err := getFlags(args)
	if err != nil {
		s.sendToolError(id, err.Error())
		return
	}
	cmdArgs = append(cmdArgs, flags...)

	if remote, ok := args["remote"].(string); ok && remote != "" {
		cmdArgs = append(cmdArgs, remote)
	}

	s.runGit(id, repoPath, cmdArgs)
}

// gitPullPush handles git pull and git push (remote + branch).
func (s *MCPServer) gitPullPush(id interface{}, args map[string]interface{}, subcmd string) {
	repoPath, ok := getRepoPath(args)
	if !ok {
		s.sendToolError(id, "repository_path is required")
		return
	}
	if err := verifyRepo(repoPath); err != nil {
		s.sendToolError(id, err.Error())
		return
	}

	cmdArgs := []string{subcmd}
	flags, err := getFlags(args)
	if err != nil {
		s.sendToolError(id, err.Error())
		return
	}
	cmdArgs = append(cmdArgs, flags...)

	if remote, ok := args["remote"].(string); ok && remote != "" {
		cmdArgs = append(cmdArgs, remote)
	}
	if branch, ok := args["branch"].(string); ok && branch != "" {
		cmdArgs = append(cmdArgs, branch)
	}

	s.runGit(id, repoPath, cmdArgs)
}

// gitClone handles git clone (no repo verification needed).
func (s *MCPServer) gitClone(id interface{}, args map[string]interface{}) {
	url, _ := args["url"].(string)
	if url == "" {
		s.sendToolError(id, "url is required")
		return
	}

	cmdArgs := []string{"clone"}
	flags, err := getFlags(args)
	if err != nil {
		s.sendToolError(id, err.Error())
		return
	}
	cmdArgs = append(cmdArgs, flags...)
	cmdArgs = append(cmdArgs, url)

	if path, ok := args["path"].(string); ok && path != "" {
		if err := validateRepoPath(path); err != nil {
			s.sendToolError(id, err.Error())
			return
		}
		cmdArgs = append(cmdArgs, path)
	}

	// Clone runs in the current working directory, not inside a repo.
	s.runGit(id, "", cmdArgs)
}

// gitTag handles git tag with optional name and message.
func (s *MCPServer) gitTag(id interface{}, args map[string]interface{}) {
	repoPath, ok := getRepoPath(args)
	if !ok {
		s.sendToolError(id, "repository_path is required")
		return
	}
	if err := verifyRepo(repoPath); err != nil {
		s.sendToolError(id, err.Error())
		return
	}

	cmdArgs := []string{"tag"}
	flags, err := getFlags(args)
	if err != nil {
		s.sendToolError(id, err.Error())
		return
	}
	cmdArgs = append(cmdArgs, flags...)

	if name, ok := args["tag_name"].(string); ok && name != "" {
		cmdArgs = append(cmdArgs, name)
	}

	if msg, ok := args["message"].(string); ok && msg != "" {
		cmdArgs = append(cmdArgs, "-m", msg)
	}

	s.runGit(id, repoPath, cmdArgs)
}

// gitStash handles git stash with subcommands.
func (s *MCPServer) gitStash(id interface{}, args map[string]interface{}) {
	repoPath, ok := getRepoPath(args)
	if !ok {
		s.sendToolError(id, "repository_path is required")
		return
	}
	if err := verifyRepo(repoPath); err != nil {
		s.sendToolError(id, err.Error())
		return
	}

	cmdArgs := []string{"stash"}

	sub, _ := args["subcommand"].(string)
	if sub != "" {
		cmdArgs = append(cmdArgs, sub)
	}

	flags, err := getFlags(args)
	if err != nil {
		s.sendToolError(id, err.Error())
		return
	}
	cmdArgs = append(cmdArgs, flags...)

	if sub == "push" || sub == "" {
		if msg, ok := args["message"].(string); ok && msg != "" {
			cmdArgs = append(cmdArgs, "-m", msg)
		}
	}

	s.runGit(id, repoPath, cmdArgs)
}

// gitInit handles git init (special: no repo verification).
func (s *MCPServer) gitInit(id interface{}, args map[string]interface{}) {
	cmdArgs := []string{"init"}
	flags, err := getFlags(args)
	if err != nil {
		s.sendToolError(id, err.Error())
		return
	}
	cmdArgs = append(cmdArgs, flags...)

	if p, ok := args["path"].(string); ok && p != "" {
		if err := validateRepoPath(p); err != nil {
			s.sendToolError(id, err.Error())
			return
		}
		cmdArgs = append(cmdArgs, p)
	}

	s.runGit(id, "", cmdArgs)
}

// gitRevParse handles git rev-parse.
func (s *MCPServer) gitRevParse(id interface{}, args map[string]interface{}) {
	repoPath, ok := getRepoPath(args)
	if !ok {
		s.sendToolError(id, "repository_path is required")
		return
	}
	if err := verifyRepo(repoPath); err != nil {
		s.sendToolError(id, err.Error())
		return
	}

	cmdArgs := []string{"rev-parse"}
	flags, err := getFlags(args)
	if err != nil {
		s.sendToolError(id, err.Error())
		return
	}
	cmdArgs = append(cmdArgs, flags...)
	cmdArgs = append(cmdArgs, getStringArray(args, "args")...)

	s.runGit(id, repoPath, cmdArgs)
}

// ---------- Git execution ----------

func (s *MCPServer) runGit(id interface{}, cwd string, gitArgs []string) {
	cmd := exec.Command("git", gitArgs...)
	if cwd != "" {
		cmd.Dir = cwd
	}

	commandStr := "git " + strings.Join(gitArgs, " ")
	logger.Printf("Executing: %s (cwd: %s)\n", commandStr, cwd)

	stdout, err := cmd.Output()
	result := GitResult{
		Command: commandStr,
		Success: err == nil,
		Stdout:  strings.TrimSpace(string(stdout)),
	}

	if err != nil {
		logger.Printf("Git command failed: %v\n", err)
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.Stderr = strings.TrimSpace(string(exitErr.Stderr))
			logger.Printf("Git stderr: %s\n", result.Stderr)
		}
		result.Error = err.Error()
	} else {
		logger.Printf("Git command succeeded, stdout length: %d bytes\n", len(result.Stdout))
	}

	data, _ := json.MarshalIndent(result, "", "  ")
	s.sendResponse(id, ToolResult{
		Content: []ContentItem{{Type: "text", Text: string(data)}},
		IsError: !result.Success,
	})
}

// ---------- Helpers ----------

func getRepoPath(args map[string]interface{}) (string, bool) {
	p, ok := args["repository_path"].(string)
	return p, ok && p != ""
}

// allowedRepoPaths restricts which directories git operations can target.
// Defaults to $HOME. Override via HUNTER3_GIT_ALLOWED_PATHS (comma-separated).
var allowedRepoPaths []string

func initAllowedPaths() {
	if envPaths := os.Getenv("HUNTER3_GIT_ALLOWED_PATHS"); envPaths != "" {
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

func verifyRepo(repoPath string) error {
	if err := validateRepoPath(repoPath); err != nil {
		return err
	}
	gitDir := filepath.Join(repoPath, ".git")
	info, err := os.Stat(gitDir)
	if err != nil {
		return fmt.Errorf("not a git repository: %s", repoPath)
	}
	// .git can be a directory (normal) or a file (worktree/submodule)
	if !info.IsDir() && info.Mode().IsRegular() {
		return nil
	}
	if info.IsDir() {
		return nil
	}
	return fmt.Errorf("not a git repository: %s", repoPath)
}

// dangerousFlagPrefixes lists git flag prefixes that can lead to arbitrary
// command execution via git subprocesses.
var dangerousFlagPrefixes = []string{
	"--exec",
	"--upload-pack",
	"--receive-pack",
	"--config",
	"-c",
	"--ext-diff",
	"--run",
}

func sanitizeFlags(flags []string) ([]string, error) {
	for _, f := range flags {
		lower := strings.ToLower(f)
		for _, prefix := range dangerousFlagPrefixes {
			if lower == prefix || strings.HasPrefix(lower, prefix+"=") {
				return nil, fmt.Errorf("flag %q is not allowed for security reasons", f)
			}
		}
	}
	return flags, nil
}

func getFlags(args map[string]interface{}) ([]string, error) {
	flags := getStringArray(args, "flags")
	return sanitizeFlags(flags)
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
