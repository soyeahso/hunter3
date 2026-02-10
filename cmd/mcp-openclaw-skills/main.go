package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	defaultSkillsPath = "~/.openclaw/skills"
)

// MCP Protocol Types
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
	Required   []string            `json:"required"`
}

type Property struct {
	Type        string      `json:"type"`
	Description string      `json:"description"`
	Default     interface{} `json:"default,omitempty"`
	Enum        []string    `json:"enum,omitempty"`
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
	Text string `json:"text,omitempty"`
}

type InitializeResult struct {
	ProtocolVersion string       `json:"protocolVersion"`
	Capabilities    Capabilities `json:"capabilities"`
	ServerInfo      ServerInfo   `json:"serverInfo"`
}

type Capabilities struct {
	Resources map[string]interface{} `json:"resources"`
	Tools     map[string]interface{} `json:"tools"`
}

type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type ListToolsResult struct {
	Tools []Tool `json:"tools"`
}

type ListResourcesResult struct {
	Resources []Resource `json:"resources"`
}

type Resource struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
}

type ReadResourceParams struct {
	URI string `json:"uri"`
}

type ResourceContents struct {
	Contents []ResourceContent `json:"contents"`
}

type ResourceContent struct {
	URI      string `json:"uri"`
	MimeType string `json:"mimeType,omitempty"`
	Text     string `json:"text,omitempty"`
}

// OpenClaw Skill Metadata Types
type SkillMetadata struct {
	Name        string                 `yaml:"name"`
	Description string                 `yaml:"description"`
	Homepage    string                 `yaml:"homepage,omitempty"`
	Metadata    map[string]interface{} `yaml:"metadata,omitempty"`
}

type OpenClawMetadata struct {
	Emoji    string                 `yaml:"emoji,omitempty"`
	OS       []string               `yaml:"os,omitempty"`
	Requires map[string]interface{} `yaml:"requires,omitempty"`
	Install  []interface{}          `yaml:"install,omitempty"`
}

var logger *log.Logger

type MCPServer struct {
	skillsPath string
	skills     map[string]*Skill
}

type Skill struct {
	Name        string
	Description string
	Content     string
	Metadata    *SkillMetadata
	Path        string
}

func initLogger() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get home dir: %v\n", err)
		return
	}
	logsDir := filepath.Join(homeDir, ".hunter3", "logs")
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create logs directory: %v\n", err)
		return
	}

	logFile := filepath.Join(logsDir, "mcp-openclaw-skills.log")
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open log file: %v\n", err)
		return
	}

	logger = log.New(io.MultiWriter(f, os.Stderr), "[mcp-openclaw-skills] ", log.LstdFlags)
	logger.Println("MCP OpenClaw Skills server starting...")
}

func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(homeDir, path[2:])
	}
	return path
}

func main() {
	initLogger()

	skillsPath := os.Getenv("OPENCLAW_SKILLS_PATH")
	if skillsPath == "" {
		skillsPath = defaultSkillsPath
	}
	skillsPath = expandPath(skillsPath)

	logger.Printf("Using skills path: %s\n", skillsPath)

	server := &MCPServer{
		skillsPath: skillsPath,
		skills:     make(map[string]*Skill),
	}

	if err := server.loadSkills(); err != nil {
		logger.Printf("Warning: Failed to load skills: %v\n", err)
	}

	logger.Println("Server initialized")
	server.Run()
}

func (s *MCPServer) loadSkills() error {
	// Check if skills directory exists
	if _, err := os.Stat(s.skillsPath); os.IsNotExist(err) {
		return fmt.Errorf("skills directory does not exist: %s", s.skillsPath)
	}

	entries, err := os.ReadDir(s.skillsPath)
	if err != nil {
		return fmt.Errorf("failed to read skills directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		skillName := entry.Name()
		skillPath := filepath.Join(s.skillsPath, skillName)
		skillFile := filepath.Join(skillPath, "SKILL.md")

		if _, err := os.Stat(skillFile); os.IsNotExist(err) {
			logger.Printf("Skipping %s: no SKILL.md found\n", skillName)
			continue
		}

		skill, err := s.loadSkill(skillName, skillFile)
		if err != nil {
			logger.Printf("Warning: Failed to load skill %s: %v\n", skillName, err)
			continue
		}

		s.skills[skillName] = skill
		logger.Printf("Loaded skill: %s\n", skillName)
	}

	logger.Printf("Loaded %d skills\n", len(s.skills))
	return nil
}

func (s *MCPServer) loadSkill(name, path string) (*Skill, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read skill file: %w", err)
	}

	skill := &Skill{
		Name:    name,
		Content: string(content),
		Path:    path,
	}

	// Parse frontmatter
	metadata, err := parseSkillMetadata(content)
	if err != nil {
		logger.Printf("Warning: Failed to parse metadata for %s: %v\n", name, err)
	} else {
		skill.Metadata = metadata
		if metadata.Description != "" {
			skill.Description = metadata.Description
		}
	}

	return skill, nil
}

func parseSkillMetadata(content []byte) (*SkillMetadata, error) {
	lines := strings.Split(string(content), "\n")
	if len(lines) < 3 || strings.TrimSpace(lines[0]) != "---" {
		return nil, fmt.Errorf("no frontmatter found")
	}

	// Find the closing ---
	endIdx := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			endIdx = i
			break
		}
	}

	if endIdx == -1 {
		return nil, fmt.Errorf("unclosed frontmatter")
	}

	frontmatter := strings.Join(lines[1:endIdx], "\n")
	var metadata SkillMetadata
	if err := yaml.Unmarshal([]byte(frontmatter), &metadata); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	return &metadata, nil
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
	case "resources/list":
		s.handleListResources(req)
	case "resources/read":
		s.handleReadResource(req)
	case "notifications/initialized":
		logger.Println("Received initialized notification")
		return
	default:
		logger.Printf("Unknown method: %s\n", req.Method)
		s.sendError(req.ID, -32601, "Method not found", fmt.Sprintf("Unknown method: %s", req.Method))
	}
}

func (s *MCPServer) handleInitialize(req JSONRPCRequest) {
	logger.Println("Handling initialize request")
	result := InitializeResult{
		ProtocolVersion: "2024-11-05",
		Capabilities: Capabilities{
			Resources: map[string]interface{}{},
			Tools:     map[string]interface{}{},
		},
		ServerInfo: ServerInfo{
			Name:    "openclaw-skills",
			Version: "1.0.0",
		},
	}

	s.sendResponse(req.ID, result)
}

func (s *MCPServer) handleListTools(req JSONRPCRequest) {
	logger.Println("Handling list tools request")
	tools := []Tool{
		{
			Name:        "list_skills",
			Description: "List all available OpenClaw skills with their names, descriptions, and metadata.",
			InputSchema: InputSchema{
				Type:       "object",
				Properties: map[string]Property{},
				Required:   []string{},
			},
		},
		{
			Name:        "get_skill",
			Description: "Get the full content and documentation for a specific OpenClaw skill.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"skill_name": {
						Type:        "string",
						Description: "The name of the skill to retrieve",
					},
				},
				Required: []string{"skill_name"},
			},
		},
		{
			Name:        "search_skills",
			Description: "Search for skills by keyword in their name, description, or content.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"query": {
						Type:        "string",
						Description: "Search query to match against skill names, descriptions, and content",
					},
					"search_content": {
						Type:        "string",
						Description: "Whether to search in skill content (true/false, default: false)",
						Default:     "false",
						Enum:        []string{"true", "false"},
					},
				},
				Required: []string{"query"},
			},
		},
	}

	result := ListToolsResult{
		Tools: tools,
	}

	s.sendResponse(req.ID, result)
}

func (s *MCPServer) handleListResources(req JSONRPCRequest) {
	logger.Println("Handling list resources request")

	var resources []Resource
	for name, skill := range s.skills {
		desc := skill.Description
		if desc == "" && skill.Metadata != nil {
			desc = skill.Metadata.Description
		}

		resources = append(resources, Resource{
			URI:         fmt.Sprintf("openclaw://skill/%s", name),
			Name:        fmt.Sprintf("OpenClaw Skill: %s", name),
			Description: desc,
			MimeType:    "text/markdown",
		})
	}

	result := ListResourcesResult{
		Resources: resources,
	}

	s.sendResponse(req.ID, result)
}

func (s *MCPServer) handleReadResource(req JSONRPCRequest) {
	var params ReadResourceParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		logger.Printf("Invalid params: %v\n", err)
		s.sendError(req.ID, -32602, "Invalid params", err.Error())
		return
	}

	logger.Printf("Reading resource: %s\n", params.URI)

	// Parse URI: openclaw://skill/{name}
	if !strings.HasPrefix(params.URI, "openclaw://skill/") {
		s.sendError(req.ID, -32602, "Invalid URI", "URI must start with openclaw://skill/")
		return
	}

	skillName := strings.TrimPrefix(params.URI, "openclaw://skill/")
	skill, ok := s.skills[skillName]
	if !ok {
		s.sendError(req.ID, -32602, "Skill not found", fmt.Sprintf("Skill '%s' not found", skillName))
		return
	}

	result := ResourceContents{
		Contents: []ResourceContent{
			{
				URI:      params.URI,
				MimeType: "text/markdown",
				Text:     skill.Content,
			},
		},
	}

	s.sendResponse(req.ID, result)
}

func (s *MCPServer) handleCallTool(req JSONRPCRequest) {
	var params CallToolParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		logger.Printf("Invalid params: %v\n", err)
		s.sendError(req.ID, -32602, "Invalid params", err.Error())
		return
	}

	logger.Printf("Calling tool: %s\n", params.Name)

	switch params.Name {
	case "list_skills":
		s.listSkills(req.ID, params.Arguments)
	case "get_skill":
		s.getSkill(req.ID, params.Arguments)
	case "search_skills":
		s.searchSkills(req.ID, params.Arguments)
	default:
		logger.Printf("Unknown tool: %s\n", params.Name)
		s.sendError(req.ID, -32602, "Unknown tool", fmt.Sprintf("Tool not found: %s", params.Name))
	}
}

func (s *MCPServer) listSkills(id interface{}, args map[string]interface{}) {
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("Available OpenClaw Skills (%d total):\n\n", len(s.skills)))

	for name, skill := range s.skills {
		desc := skill.Description
		if desc == "" && skill.Metadata != nil {
			desc = skill.Metadata.Description
		}

		emoji := ""
		if skill.Metadata != nil && skill.Metadata.Metadata != nil {
			if ocMeta, ok := skill.Metadata.Metadata["openclaw"].(map[string]interface{}); ok {
				if e, ok := ocMeta["emoji"].(string); ok {
					emoji = e + " "
				}
			}
		}

		builder.WriteString(fmt.Sprintf("%s**%s**: %s\n", emoji, name, desc))
	}

	s.sendToolResult(id, builder.String())
}

func (s *MCPServer) getSkill(id interface{}, args map[string]interface{}) {
	skillName, ok := args["skill_name"].(string)
	if !ok || skillName == "" {
		s.sendToolError(id, "skill_name parameter is required")
		return
	}

	skill, ok := s.skills[skillName]
	if !ok {
		s.sendToolError(id, fmt.Sprintf("Skill '%s' not found", skillName))
		return
	}

	s.sendToolResult(id, skill.Content)
}

func (s *MCPServer) searchSkills(id interface{}, args map[string]interface{}) {
	query, ok := args["query"].(string)
	if !ok || query == "" {
		s.sendToolError(id, "query parameter is required")
		return
	}

	searchContent := false
	if sc, ok := args["search_content"].(string); ok {
		searchContent = (sc == "true")
	}

	query = strings.ToLower(query)
	var matches []string

	for name, skill := range s.skills {
		matched := false

		// Search in name
		if strings.Contains(strings.ToLower(name), query) {
			matched = true
		}

		// Search in description
		if !matched && skill.Description != "" && strings.Contains(strings.ToLower(skill.Description), query) {
			matched = true
		}

		// Search in metadata description
		if !matched && skill.Metadata != nil && strings.Contains(strings.ToLower(skill.Metadata.Description), query) {
			matched = true
		}

		// Search in content if requested
		if !matched && searchContent && strings.Contains(strings.ToLower(skill.Content), query) {
			matched = true
		}

		if matched {
			desc := skill.Description
			if desc == "" && skill.Metadata != nil {
				desc = skill.Metadata.Description
			}

			emoji := ""
			if skill.Metadata != nil && skill.Metadata.Metadata != nil {
				if ocMeta, ok := skill.Metadata.Metadata["openclaw"].(map[string]interface{}); ok {
					if e, ok := ocMeta["emoji"].(string); ok {
						emoji = e + " "
					}
				}
			}

			matches = append(matches, fmt.Sprintf("%s**%s**: %s", emoji, name, desc))
		}
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Found %d skills matching '%s':\n\n", len(matches), query))
	for _, match := range matches {
		result.WriteString(match + "\n")
	}

	if len(matches) == 0 {
		result.WriteString("No skills found matching the query.\n")
	}

	s.sendToolResult(id, result.String())
}

func (s *MCPServer) sendToolResult(id interface{}, text string) {
	result := ToolResult{
		Content: []ContentItem{
			{Type: "text", Text: text},
		},
	}
	s.sendResponse(id, result)
}

func (s *MCPServer) sendToolError(id interface{}, text string) {
	result := ToolResult{
		Content: []ContentItem{
			{Type: "text", Text: text},
		},
		IsError: true,
	}
	s.sendResponse(id, result)
}

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
		Error: &RPCError{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}

	jsonData, err := json.Marshal(resp)
	if err != nil {
		logger.Printf("Error marshaling error response: %v\n", err)
		fmt.Fprintf(os.Stderr, "Error marshaling error response: %v\n", err)
		return
	}

	fmt.Println(string(jsonData))
}
