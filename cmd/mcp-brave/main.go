package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	braveSearchURL = "https://api.search.brave.com/res/v1/web/search"
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
	Tools map[string]interface{} `json:"tools"`
}

type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type ListToolsResult struct {
	Tools []Tool `json:"tools"`
}

type BraveSearchResponse struct {
	Query struct {
		Original string `json:"original"`
	} `json:"query"`
	Web struct {
		Results []struct {
			Title       string `json:"title"`
			URL         string `json:"url"`
			Description string `json:"description"`
			Age         string `json:"age,omitempty"`
		} `json:"results"`
	} `json:"web"`
	News struct {
		Results []struct {
			Title       string `json:"title"`
			URL         string `json:"url"`
			Description string `json:"description"`
			Age         string `json:"age,omitempty"`
			Source      string `json:"source,omitempty"`
		} `json:"results,omitempty"`
	} `json:"news,omitempty"`
}

var logger *log.Logger

type MCPServer struct {
	apiKey string
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

	logFile := filepath.Join(logsDir, "mcp-brave.log")
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open log file: %v\n", err)
		return
	}

	logger = log.New(io.MultiWriter(f, os.Stderr), "[mcp-brave] ", log.LstdFlags)
	logger.Println("MCP Brave Search server starting...")
}

func main() {
	initLogger()

	apiKey := os.Getenv("BRAVE_API_KEY")
	if apiKey == "" {
		logger.Fatal("BRAVE_API_KEY environment variable not set")
	}

	server := &MCPServer{apiKey: apiKey}
	logger.Println("Server initialized")
	server.Run()
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
			Tools: map[string]interface{}{},
		},
		ServerInfo: ServerInfo{
			Name:    "brave-search",
			Version: "1.0.0",
		},
	}

	s.sendResponse(req.ID, result)
}

func (s *MCPServer) handleListTools(req JSONRPCRequest) {
	logger.Println("Handling list tools request")
	tools := []Tool{
		{
			Name:        "brave_web_search",
			Description: "Search the web using Brave Search API. Returns web search results with titles, URLs, and descriptions.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"query": {
						Type:        "string",
						Description: "The search query",
					},
					"count": {
						Type:        "number",
						Description: "Number of results to return (1-20, default: 10)",
						Default:     10,
					},
					"country": {
						Type:        "string",
						Description: "Country code for search results (e.g., 'us', 'uk', 'ca'). Default: 'us'",
						Default:     "us",
					},
				},
				Required: []string{"query"},
			},
		},
		{
			Name:        "brave_news_search",
			Description: "Search for news articles using Brave Search API. Returns recent news articles with titles, URLs, descriptions, and sources.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"query": {
						Type:        "string",
						Description: "The news search query",
					},
					"count": {
						Type:        "number",
						Description: "Number of results to return (1-20, default: 10)",
						Default:     10,
					},
					"country": {
						Type:        "string",
						Description: "Country code for news results (e.g., 'us', 'uk', 'ca'). Default: 'us'",
						Default:     "us",
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

func (s *MCPServer) handleCallTool(req JSONRPCRequest) {
	var params CallToolParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		logger.Printf("Invalid params: %v\n", err)
		s.sendError(req.ID, -32602, "Invalid params", err.Error())
		return
	}

	logger.Printf("Calling tool: %s\n", params.Name)

	switch params.Name {
	case "brave_web_search":
		s.braveWebSearch(req.ID, params.Arguments)
	case "brave_news_search":
		s.braveNewsSearch(req.ID, params.Arguments)
	default:
		logger.Printf("Unknown tool: %s\n", params.Name)
		s.sendError(req.ID, -32602, "Unknown tool", fmt.Sprintf("Tool not found: %s", params.Name))
	}
}

func (s *MCPServer) braveWebSearch(id interface{}, args map[string]interface{}) {
	query, _ := args["query"].(string)
	if query == "" {
		s.sendToolError(id, "query parameter is required")
		return
	}

	count := 10
	if c, ok := args["count"].(float64); ok && c >= 1 && c <= 20 {
		count = int(c)
	}

	country := "us"
	if c, ok := args["country"].(string); ok && c != "" {
		country = c
	}

	results, err := performBraveSearch(s.apiKey, query, count, country, false)
	if err != nil {
		logger.Printf("Search error: %v", err)
		s.sendToolError(id, fmt.Sprintf("search failed: %v", err))
		return
	}

	s.sendToolResult(id, results)
}

func (s *MCPServer) braveNewsSearch(id interface{}, args map[string]interface{}) {
	query, _ := args["query"].(string)
	if query == "" {
		s.sendToolError(id, "query parameter is required")
		return
	}

	count := 10
	if c, ok := args["count"].(float64); ok && c >= 1 && c <= 20 {
		count = int(c)
	}

	country := "us"
	if c, ok := args["country"].(string); ok && c != "" {
		country = c
	}

	results, err := performBraveSearch(s.apiKey, query, count, country, true)
	if err != nil {
		logger.Printf("News search error: %v", err)
		s.sendToolError(id, fmt.Sprintf("search failed: %v", err))
		return
	}

	s.sendToolResult(id, results)
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

func performBraveSearch(apiKey, query string, count int, country string, newsOnly bool) (string, error) {
	params := url.Values{}
	params.Set("q", query)
	params.Set("count", fmt.Sprintf("%d", count))
	params.Set("country", country)
	if newsOnly {
		params.Set("search_lang", "en")
		params.Set("freshness", "pw") // Past week for news
	}

	searchURL := braveSearchURL + "?" + params.Encode()

	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Subscription-Token", apiKey)

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var searchResp BraveSearchResponse
	if err := json.Unmarshal(body, &searchResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	return formatResults(searchResp, newsOnly), nil
}

func formatResults(resp BraveSearchResponse, newsOnly bool) string {
	var builder strings.Builder

	if newsOnly {
		builder.WriteString(fmt.Sprintf("News search results for: %s\n\n", resp.Query.Original))
		if len(resp.News.Results) == 0 {
			builder.WriteString("No news results found.\n")
			return builder.String()
		}

		for i, result := range resp.News.Results {
			builder.WriteString(fmt.Sprintf("%d. %s\n", i+1, result.Title))
			builder.WriteString(fmt.Sprintf("   URL: %s\n", result.URL))
			if result.Source != "" {
				builder.WriteString(fmt.Sprintf("   Source: %s\n", result.Source))
			}
			if result.Age != "" {
				builder.WriteString(fmt.Sprintf("   Age: %s\n", result.Age))
			}
			builder.WriteString(fmt.Sprintf("   %s\n\n", result.Description))
		}
	} else {
		builder.WriteString(fmt.Sprintf("Web search results for: %s\n\n", resp.Query.Original))
		if len(resp.Web.Results) == 0 {
			builder.WriteString("No web results found.\n")
			return builder.String()
		}

		for i, result := range resp.Web.Results {
			builder.WriteString(fmt.Sprintf("%d. %s\n", i+1, result.Title))
			builder.WriteString(fmt.Sprintf("   URL: %s\n", result.URL))
			if result.Age != "" {
				builder.WriteString(fmt.Sprintf("   Age: %s\n", result.Age))
			}
			builder.WriteString(fmt.Sprintf("   %s\n\n", result.Description))
		}
	}

	return builder.String()
}
