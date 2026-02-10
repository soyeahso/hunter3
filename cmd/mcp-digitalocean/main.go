package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/digitalocean/godo"
	"golang.org/x/oauth2"
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

// TokenSource implements oauth2.TokenSource
type TokenSource struct {
	AccessToken string
}

func (t *TokenSource) Token() (*oauth2.Token, error) {
	token := &oauth2.Token{
		AccessToken: t.AccessToken,
	}
	return token, nil
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

func boolProp(desc string) Property {
	return Property{Type: "boolean", Description: desc}
}

func numberProp(desc string) Property {
	return Property{Type: "number", Description: desc}
}

// MCPServer handles the JSON-RPC stdin/stdout protocol.
type MCPServer struct {
	client *godo.Client
}

var logger *log.Logger

func initLogger() {
	// Create logs directory if it doesn't exist
	logsDir := filepath.Join(os.Getenv("HOME"), ".hunter3", "logs")
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create logs directory: %v\n", err)
		return
	}

	// Open log file
	logFile := filepath.Join(logsDir, "mcp-digitalocean.log")
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open log file: %v\n", err)
		return
	}

	// Create logger that writes to both file and stderr
	logger = log.New(io.MultiWriter(f, os.Stderr), "[mcp-digitalocean] ", log.LstdFlags)
	logger.Println("MCP DigitalOcean server starting...")
}

func main() {
	initLogger()

	// Get DigitalOcean API token from environment
	token := os.Getenv("DIGITALOCEAN_TOKEN")
	if token == "" {
		logger.Fatal("DIGITALOCEAN_TOKEN environment variable not set")
	}

	// Create OAuth2 token source
	tokenSource := &TokenSource{AccessToken: token}
	oauthClient := oauth2.NewClient(context.Background(), tokenSource)

	// Create DigitalOcean client
	client := godo.NewClient(oauthClient)

	s := &MCPServer{client: client}
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
		ServerInfo:      ServerInfo{Name: "mcp-digitalocean", Version: "1.0.0"},
	})
}

// ---------- Tool definitions ----------

func (s *MCPServer) handleListTools(req JSONRPCRequest) {
	logger.Println("Handling list tools request")

	tools := []Tool{
		// --- Droplet (VM) Management ---
		{
			Name:        "list_droplets",
			Description: "List all Droplets (VMs) in your DigitalOcean account",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"tag": stringProp("Filter droplets by tag name"),
				},
			},
		},
		{
			Name:        "get_droplet",
			Description: "Get detailed information about a specific Droplet by ID",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"droplet_id": numberProp("The ID of the Droplet"),
				},
				Required: []string{"droplet_id"},
			},
		},
		{
			Name:        "create_droplet",
			Description: "Create a new Droplet (VM). Common images: ubuntu-24-04-x64, ubuntu-22-04-x64, debian-12-x64, fedora-40-x64. Common sizes: s-1vcpu-1gb, s-1vcpu-2gb, s-2vcpu-2gb, s-2vcpu-4gb",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"name":       stringProp("Name for the Droplet"),
					"region":     stringPropDefault("Region slug (e.g., 'nyc1', 'nyc3', 'sfo3', 'lon1', 'ams3')", "nyc3"),
					"size":       stringPropDefault("Size slug (e.g., 's-1vcpu-1gb', 's-2vcpu-2gb')", "s-1vcpu-1gb"),
					"image":      stringPropDefault("Image slug (e.g., 'ubuntu-24-04-x64', 'debian-12-x64')", "ubuntu-24-04-x64"),
					"ssh_keys":   stringArrayProp("Array of SSH key IDs or fingerprints to add to the Droplet"),
					"backups":    boolProp("Enable automated backups"),
					"ipv6":       boolProp("Enable IPv6"),
					"monitoring": boolProp("Enable monitoring"),
					"tags":       stringArrayProp("Tags to apply to the Droplet"),
					"user_data":  stringProp("User data (cloud-init script) to run on first boot"),
					"vpc_uuid":   stringProp("UUID of the VPC to create the Droplet in"),
				},
				Required: []string{"name", "region", "size", "image"},
			},
		},
		{
			Name:        "delete_droplet",
			Description: "Delete (destroy) a Droplet by ID",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"droplet_id": numberProp("The ID of the Droplet to delete"),
				},
				Required: []string{"droplet_id"},
			},
		},
		{
			Name:        "power_on_droplet",
			Description: "Power on a Droplet",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"droplet_id": numberProp("The ID of the Droplet to power on"),
				},
				Required: []string{"droplet_id"},
			},
		},
		{
			Name:        "power_off_droplet",
			Description: "Power off a Droplet (graceful shutdown)",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"droplet_id": numberProp("The ID of the Droplet to power off"),
				},
				Required: []string{"droplet_id"},
			},
		},
		{
			Name:        "reboot_droplet",
			Description: "Reboot a Droplet (graceful reboot)",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"droplet_id": numberProp("The ID of the Droplet to reboot"),
				},
				Required: []string{"droplet_id"},
			},
		},
		{
			Name:        "shutdown_droplet",
			Description: "Shutdown a Droplet (send ACPI shutdown signal)",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"droplet_id": numberProp("The ID of the Droplet to shutdown"),
				},
				Required: []string{"droplet_id"},
			},
		},
		{
			Name:        "power_cycle_droplet",
			Description: "Power cycle a Droplet (hard reset)",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"droplet_id": numberProp("The ID of the Droplet to power cycle"),
				},
				Required: []string{"droplet_id"},
			},
		},
		{
			Name:        "resize_droplet",
			Description: "Resize a Droplet to a different size slug",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"droplet_id": numberProp("The ID of the Droplet to resize"),
					"size":       stringProp("New size slug (e.g., 's-2vcpu-4gb')"),
					"disk":       boolProp("Resize the disk (permanent, cannot be reversed)"),
				},
				Required: []string{"droplet_id", "size"},
			},
		},
		{
			Name:        "snapshot_droplet",
			Description: "Create a snapshot of a Droplet",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"droplet_id":    numberProp("The ID of the Droplet to snapshot"),
					"snapshot_name": stringProp("Name for the snapshot"),
				},
				Required: []string{"droplet_id", "snapshot_name"},
			},
		},
		{
			Name:        "get_droplet_action",
			Description: "Get the status of a Droplet action by action ID",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"droplet_id": numberProp("The ID of the Droplet"),
					"action_id":  numberProp("The ID of the action"),
				},
				Required: []string{"droplet_id", "action_id"},
			},
		},

		// --- SSH Keys ---
		{
			Name:        "list_ssh_keys",
			Description: "List all SSH keys in your DigitalOcean account",
			InputSchema: InputSchema{
				Type:       "object",
				Properties: map[string]Property{},
			},
		},
		{
			Name:        "create_ssh_key",
			Description: "Add a new SSH key to your DigitalOcean account",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"name":       stringProp("Name for the SSH key"),
					"public_key": stringProp("The public key string"),
				},
				Required: []string{"name", "public_key"},
			},
		},
		{
			Name:        "delete_ssh_key",
			Description: "Delete an SSH key by ID or fingerprint",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"key_id": stringProp("The SSH key ID or fingerprint"),
				},
				Required: []string{"key_id"},
			},
		},

		// --- Regions ---
		{
			Name:        "list_regions",
			Description: "List all available DigitalOcean regions",
			InputSchema: InputSchema{
				Type:       "object",
				Properties: map[string]Property{},
			},
		},

		// --- Sizes ---
		{
			Name:        "list_sizes",
			Description: "List all available Droplet sizes",
			InputSchema: InputSchema{
				Type:       "object",
				Properties: map[string]Property{},
			},
		},

		// --- Images ---
		{
			Name:        "list_images",
			Description: "List available images (distributions, snapshots, backups)",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"type": stringProp("Filter by type: 'distribution', 'application', or omit for all"),
				},
			},
		},

		// --- Tags ---
		{
			Name:        "list_tags",
			Description: "List all tags in your DigitalOcean account",
			InputSchema: InputSchema{
				Type:       "object",
				Properties: map[string]Property{},
			},
		},
		{
			Name:        "create_tag",
			Description: "Create a new tag",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"name": stringProp("Name for the tag"),
				},
				Required: []string{"name"},
			},
		},
		{
			Name:        "delete_tag",
			Description: "Delete a tag",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"name": stringProp("Name of the tag to delete"),
				},
				Required: []string{"name"},
			},
		},
		{
			Name:        "tag_resources",
			Description: "Tag resources (Droplets, images, volumes, etc.)",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"tag":       stringProp("Tag name"),
					"resources": stringArrayProp("Array of resource URNs (e.g., 'do:droplet:12345')"),
				},
				Required: []string{"tag", "resources"},
			},
		},
		{
			Name:        "untag_resources",
			Description: "Remove tag from resources",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"tag":       stringProp("Tag name"),
					"resources": stringArrayProp("Array of resource URNs to untag"),
				},
				Required: []string{"tag", "resources"},
			},
		},

		// --- Account ---
		{
			Name:        "get_account",
			Description: "Get your DigitalOcean account information",
			InputSchema: InputSchema{
				Type:       "object",
				Properties: map[string]Property{},
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
	ctx := context.Background()

	switch params.Name {
	// Droplet commands
	case "list_droplets":
		s.listDroplets(ctx, req.ID, args)
	case "get_droplet":
		s.getDroplet(ctx, req.ID, args)
	case "create_droplet":
		s.createDroplet(ctx, req.ID, args)
	case "delete_droplet":
		s.deleteDroplet(ctx, req.ID, args)
	case "power_on_droplet":
		s.dropletAction(ctx, req.ID, args, "power_on")
	case "power_off_droplet":
		s.dropletAction(ctx, req.ID, args, "power_off")
	case "reboot_droplet":
		s.dropletAction(ctx, req.ID, args, "reboot")
	case "shutdown_droplet":
		s.dropletAction(ctx, req.ID, args, "shutdown")
	case "power_cycle_droplet":
		s.dropletAction(ctx, req.ID, args, "power_cycle")
	case "resize_droplet":
		s.resizeDroplet(ctx, req.ID, args)
	case "snapshot_droplet":
		s.snapshotDroplet(ctx, req.ID, args)
	case "get_droplet_action":
		s.getDropletAction(ctx, req.ID, args)

	// SSH key commands
	case "list_ssh_keys":
		s.listSSHKeys(ctx, req.ID, args)
	case "create_ssh_key":
		s.createSSHKey(ctx, req.ID, args)
	case "delete_ssh_key":
		s.deleteSSHKey(ctx, req.ID, args)

	// Region commands
	case "list_regions":
		s.listRegions(ctx, req.ID, args)

	// Size commands
	case "list_sizes":
		s.listSizes(ctx, req.ID, args)

	// Image commands
	case "list_images":
		s.listImages(ctx, req.ID, args)

	// Tag commands
	case "list_tags":
		s.listTags(ctx, req.ID, args)
	case "create_tag":
		s.createTag(ctx, req.ID, args)
	case "delete_tag":
		s.deleteTag(ctx, req.ID, args)
	case "tag_resources":
		s.tagResources(ctx, req.ID, args)
	case "untag_resources":
		s.untagResources(ctx, req.ID, args)

	// Account commands
	case "get_account":
		s.getAccount(ctx, req.ID, args)

	default:
		s.sendToolError(req.ID, fmt.Sprintf("Unknown tool: %s", params.Name))
	}
}

// ---------- Droplet Tool Handlers ----------

func (s *MCPServer) listDroplets(ctx context.Context, id interface{}, args map[string]interface{}) {
	opt := &godo.ListOptions{PerPage: 200}
	tag := getString(args, "tag")

	var allDroplets []godo.Droplet

	for {
		var droplets []godo.Droplet
		var resp *godo.Response
		var err error

		if tag != "" {
			droplets, resp, err = s.client.Droplets.ListByTag(ctx, tag, opt)
		} else {
			droplets, resp, err = s.client.Droplets.List(ctx, opt)
		}

		if err != nil {
			s.sendToolError(id, fmt.Sprintf("Failed to list droplets: %v", err))
			return
		}

		allDroplets = append(allDroplets, droplets...)

		if resp.Links == nil || resp.Links.IsLastPage() {
			break
		}

		page, err := resp.Links.CurrentPage()
		if err != nil {
			break
		}
		opt.Page = page + 1
	}

	s.sendJSONResponse(id, allDroplets)
}

func (s *MCPServer) getDroplet(ctx context.Context, id interface{}, args map[string]interface{}) {
	dropletID := getInt(args, "droplet_id")
	if dropletID == 0 {
		s.sendToolError(id, "droplet_id is required")
		return
	}

	droplet, _, err := s.client.Droplets.Get(ctx, dropletID)
	if err != nil {
		s.sendToolError(id, fmt.Sprintf("Failed to get droplet: %v", err))
		return
	}

	s.sendJSONResponse(id, droplet)
}

func (s *MCPServer) createDroplet(ctx context.Context, id interface{}, args map[string]interface{}) {
	name := getString(args, "name")
	region := getString(args, "region")
	size := getString(args, "size")
	image := getString(args, "image")

	if name == "" || region == "" || size == "" || image == "" {
		s.sendToolError(id, "name, region, size, and image are required")
		return
	}

	createRequest := &godo.DropletCreateRequest{
		Name:   name,
		Region: region,
		Size:   size,
		Image: godo.DropletCreateImage{
			Slug: image,
		},
		Backups:    getBool(args, "backups"),
		IPv6:       getBool(args, "ipv6"),
		Monitoring: getBool(args, "monitoring"),
		Tags:       getStringArray(args, "tags"),
		UserData:   getString(args, "user_data"),
		VPCUUID:    getString(args, "vpc_uuid"),
	}

	// Handle SSH keys
	sshKeys := getStringArray(args, "ssh_keys")
	if len(sshKeys) > 0 {
		createRequest.SSHKeys = make([]godo.DropletCreateSSHKey, len(sshKeys))
		for i, key := range sshKeys {
			// Try to parse as int (ID), otherwise use as fingerprint
			if keyID, err := strconv.Atoi(key); err == nil {
				createRequest.SSHKeys[i] = godo.DropletCreateSSHKey{ID: keyID}
			} else {
				createRequest.SSHKeys[i] = godo.DropletCreateSSHKey{Fingerprint: key}
			}
		}
	}

	droplet, _, err := s.client.Droplets.Create(ctx, createRequest)
	if err != nil {
		s.sendToolError(id, fmt.Sprintf("Failed to create droplet: %v", err))
		return
	}

	s.sendJSONResponse(id, droplet)
}

func (s *MCPServer) deleteDroplet(ctx context.Context, id interface{}, args map[string]interface{}) {
	dropletID := getInt(args, "droplet_id")
	if dropletID == 0 {
		s.sendToolError(id, "droplet_id is required")
		return
	}

	_, err := s.client.Droplets.Delete(ctx, dropletID)
	if err != nil {
		s.sendToolError(id, fmt.Sprintf("Failed to delete droplet: %v", err))
		return
	}

	s.sendJSONResponse(id, map[string]string{"status": "deleted", "droplet_id": fmt.Sprintf("%d", dropletID)})
}

func (s *MCPServer) dropletAction(ctx context.Context, id interface{}, args map[string]interface{}, actionType string) {
	dropletID := getInt(args, "droplet_id")
	if dropletID == 0 {
		s.sendToolError(id, "droplet_id is required")
		return
	}

	var action *godo.Action
	var err error

	switch actionType {
	case "power_on":
		action, _, err = s.client.DropletActions.PowerOn(ctx, dropletID)
	case "power_off":
		action, _, err = s.client.DropletActions.PowerOff(ctx, dropletID)
	case "reboot":
		action, _, err = s.client.DropletActions.Reboot(ctx, dropletID)
	case "shutdown":
		action, _, err = s.client.DropletActions.Shutdown(ctx, dropletID)
	case "power_cycle":
		action, _, err = s.client.DropletActions.PowerCycle(ctx, dropletID)
	default:
		s.sendToolError(id, fmt.Sprintf("Unknown action type: %s", actionType))
		return
	}

	if err != nil {
		s.sendToolError(id, fmt.Sprintf("Failed to %s droplet: %v", actionType, err))
		return
	}

	s.sendJSONResponse(id, action)
}

func (s *MCPServer) resizeDroplet(ctx context.Context, id interface{}, args map[string]interface{}) {
	dropletID := getInt(args, "droplet_id")
	size := getString(args, "size")

	if dropletID == 0 || size == "" {
		s.sendToolError(id, "droplet_id and size are required")
		return
	}

	disk := getBool(args, "disk")
	action, _, err := s.client.DropletActions.Resize(ctx, dropletID, size, disk)
	if err != nil {
		s.sendToolError(id, fmt.Sprintf("Failed to resize droplet: %v", err))
		return
	}

	s.sendJSONResponse(id, action)
}

func (s *MCPServer) snapshotDroplet(ctx context.Context, id interface{}, args map[string]interface{}) {
	dropletID := getInt(args, "droplet_id")
	snapshotName := getString(args, "snapshot_name")

	if dropletID == 0 || snapshotName == "" {
		s.sendToolError(id, "droplet_id and snapshot_name are required")
		return
	}

	action, _, err := s.client.DropletActions.Snapshot(ctx, dropletID, snapshotName)
	if err != nil {
		s.sendToolError(id, fmt.Sprintf("Failed to snapshot droplet: %v", err))
		return
	}

	s.sendJSONResponse(id, action)
}

func (s *MCPServer) getDropletAction(ctx context.Context, id interface{}, args map[string]interface{}) {
	dropletID := getInt(args, "droplet_id")
	actionID := getInt(args, "action_id")

	if dropletID == 0 || actionID == 0 {
		s.sendToolError(id, "droplet_id and action_id are required")
		return
	}

	action, _, err := s.client.DropletActions.Get(ctx, dropletID, actionID)
	if err != nil {
		s.sendToolError(id, fmt.Sprintf("Failed to get action: %v", err))
		return
	}

	s.sendJSONResponse(id, action)
}

// ---------- SSH Key Tool Handlers ----------

func (s *MCPServer) listSSHKeys(ctx context.Context, id interface{}, args map[string]interface{}) {
	opt := &godo.ListOptions{PerPage: 200}
	var allKeys []godo.Key

	for {
		keys, resp, err := s.client.Keys.List(ctx, opt)
		if err != nil {
			s.sendToolError(id, fmt.Sprintf("Failed to list SSH keys: %v", err))
			return
		}

		allKeys = append(allKeys, keys...)

		if resp.Links == nil || resp.Links.IsLastPage() {
			break
		}

		page, err := resp.Links.CurrentPage()
		if err != nil {
			break
		}
		opt.Page = page + 1
	}

	s.sendJSONResponse(id, allKeys)
}

func (s *MCPServer) createSSHKey(ctx context.Context, id interface{}, args map[string]interface{}) {
	name := getString(args, "name")
	publicKey := getString(args, "public_key")

	if name == "" || publicKey == "" {
		s.sendToolError(id, "name and public_key are required")
		return
	}

	createRequest := &godo.KeyCreateRequest{
		Name:      name,
		PublicKey: publicKey,
	}

	key, _, err := s.client.Keys.Create(ctx, createRequest)
	if err != nil {
		s.sendToolError(id, fmt.Sprintf("Failed to create SSH key: %v", err))
		return
	}

	s.sendJSONResponse(id, key)
}

func (s *MCPServer) deleteSSHKey(ctx context.Context, id interface{}, args map[string]interface{}) {
	keyID := getString(args, "key_id")
	if keyID == "" {
		s.sendToolError(id, "key_id is required")
		return
	}

	_, err := s.client.Keys.DeleteByID(ctx, getInt(args, "key_id"))
	if err != nil {
		// Try by fingerprint
		_, err = s.client.Keys.DeleteByFingerprint(ctx, keyID)
		if err != nil {
			s.sendToolError(id, fmt.Sprintf("Failed to delete SSH key: %v", err))
			return
		}
	}

	s.sendJSONResponse(id, map[string]string{"status": "deleted", "key_id": keyID})
}

// ---------- Region Tool Handlers ----------

func (s *MCPServer) listRegions(ctx context.Context, id interface{}, args map[string]interface{}) {
	opt := &godo.ListOptions{PerPage: 200}
	var allRegions []godo.Region

	for {
		regions, resp, err := s.client.Regions.List(ctx, opt)
		if err != nil {
			s.sendToolError(id, fmt.Sprintf("Failed to list regions: %v", err))
			return
		}

		allRegions = append(allRegions, regions...)

		if resp.Links == nil || resp.Links.IsLastPage() {
			break
		}

		page, err := resp.Links.CurrentPage()
		if err != nil {
			break
		}
		opt.Page = page + 1
	}

	s.sendJSONResponse(id, allRegions)
}

// ---------- Size Tool Handlers ----------

func (s *MCPServer) listSizes(ctx context.Context, id interface{}, args map[string]interface{}) {
	opt := &godo.ListOptions{PerPage: 200}
	var allSizes []godo.Size

	for {
		sizes, resp, err := s.client.Sizes.List(ctx, opt)
		if err != nil {
			s.sendToolError(id, fmt.Sprintf("Failed to list sizes: %v", err))
			return
		}

		allSizes = append(allSizes, sizes...)

		if resp.Links == nil || resp.Links.IsLastPage() {
			break
		}

		page, err := resp.Links.CurrentPage()
		if err != nil {
			break
		}
		opt.Page = page + 1
	}

	s.sendJSONResponse(id, allSizes)
}

// ---------- Image Tool Handlers ----------

func (s *MCPServer) listImages(ctx context.Context, id interface{}, args map[string]interface{}) {
	opt := &godo.ListOptions{PerPage: 200}
	imageType := getString(args, "type")

	var allImages []godo.Image

	for {
		images, resp, err := s.client.Images.List(ctx, opt)
		if err != nil {
			s.sendToolError(id, fmt.Sprintf("Failed to list images: %v", err))
			return
		}

		// Filter by type if specified
		if imageType != "" {
			for _, img := range images {
				if img.Type == imageType {
					allImages = append(allImages, img)
				}
			}
		} else {
			allImages = append(allImages, images...)
		}

		if resp.Links == nil || resp.Links.IsLastPage() {
			break
		}

		page, err := resp.Links.CurrentPage()
		if err != nil {
			break
		}
		opt.Page = page + 1
	}

	s.sendJSONResponse(id, allImages)
}

// ---------- Tag Tool Handlers ----------

func (s *MCPServer) listTags(ctx context.Context, id interface{}, args map[string]interface{}) {
	opt := &godo.ListOptions{PerPage: 200}
	var allTags []godo.Tag

	for {
		tags, resp, err := s.client.Tags.List(ctx, opt)
		if err != nil {
			s.sendToolError(id, fmt.Sprintf("Failed to list tags: %v", err))
			return
		}

		allTags = append(allTags, tags...)

		if resp.Links == nil || resp.Links.IsLastPage() {
			break
		}

		page, err := resp.Links.CurrentPage()
		if err != nil {
			break
		}
		opt.Page = page + 1
	}

	s.sendJSONResponse(id, allTags)
}

func (s *MCPServer) createTag(ctx context.Context, id interface{}, args map[string]interface{}) {
	name := getString(args, "name")
	if name == "" {
		s.sendToolError(id, "name is required")
		return
	}

	createRequest := &godo.TagCreateRequest{
		Name: name,
	}

	tag, _, err := s.client.Tags.Create(ctx, createRequest)
	if err != nil {
		s.sendToolError(id, fmt.Sprintf("Failed to create tag: %v", err))
		return
	}

	s.sendJSONResponse(id, tag)
}

func (s *MCPServer) deleteTag(ctx context.Context, id interface{}, args map[string]interface{}) {
	name := getString(args, "name")
	if name == "" {
		s.sendToolError(id, "name is required")
		return
	}

	_, err := s.client.Tags.Delete(ctx, name)
	if err != nil {
		s.sendToolError(id, fmt.Sprintf("Failed to delete tag: %v", err))
		return
	}

	s.sendJSONResponse(id, map[string]string{"status": "deleted", "tag": name})
}

func (s *MCPServer) tagResources(ctx context.Context, id interface{}, args map[string]interface{}) {
	tagName := getString(args, "tag")
	resources := getStringArray(args, "resources")

	if tagName == "" || len(resources) == 0 {
		s.sendToolError(id, "tag and resources are required")
		return
	}

	tagRequest := &godo.TagResourcesRequest{
		Resources: make([]godo.Resource, len(resources)),
	}

	for i, urn := range resources {
		// Parse URN format: do:droplet:12345
		parts := strings.Split(urn, ":")
		if len(parts) != 3 {
			s.sendToolError(id, fmt.Sprintf("Invalid resource URN format: %s (expected format: do:type:id)", urn))
			return
		}
		tagRequest.Resources[i] = godo.Resource{
			ID:   parts[2],
			Type: godo.ResourceType(parts[1]),
		}
	}

	_, err := s.client.Tags.TagResources(ctx, tagName, tagRequest)
	if err != nil {
		s.sendToolError(id, fmt.Sprintf("Failed to tag resources: %v", err))
		return
	}

	s.sendJSONResponse(id, map[string]interface{}{
		"status":    "tagged",
		"tag":       tagName,
		"resources": resources,
	})
}

func (s *MCPServer) untagResources(ctx context.Context, id interface{}, args map[string]interface{}) {
	tagName := getString(args, "tag")
	resources := getStringArray(args, "resources")

	if tagName == "" || len(resources) == 0 {
		s.sendToolError(id, "tag and resources are required")
		return
	}

	untagRequest := &godo.UntagResourcesRequest{
		Resources: make([]godo.Resource, len(resources)),
	}

	for i, urn := range resources {
		parts := strings.Split(urn, ":")
		if len(parts) != 3 {
			s.sendToolError(id, fmt.Sprintf("Invalid resource URN format: %s", urn))
			return
		}
		untagRequest.Resources[i] = godo.Resource{
			ID:   parts[2],
			Type: godo.ResourceType(parts[1]),
		}
	}

	_, err := s.client.Tags.UntagResources(ctx, tagName, untagRequest)
	if err != nil {
		s.sendToolError(id, fmt.Sprintf("Failed to untag resources: %v", err))
		return
	}

	s.sendJSONResponse(id, map[string]interface{}{
		"status":    "untagged",
		"tag":       tagName,
		"resources": resources,
	})
}

// ---------- Account Tool Handlers ----------

func (s *MCPServer) getAccount(ctx context.Context, id interface{}, args map[string]interface{}) {
	account, _, err := s.client.Account.Get(ctx)
	if err != nil {
		s.sendToolError(id, fmt.Sprintf("Failed to get account info: %v", err))
		return
	}

	s.sendJSONResponse(id, account)
}

// ---------- Helpers ----------

func getString(args map[string]interface{}, key string) string {
	if val, ok := args[key].(string); ok {
		return val
	}
	return ""
}

func getBool(args map[string]interface{}, key string) bool {
	if val, ok := args[key].(bool); ok {
		return val
	}
	return false
}

func getInt(args map[string]interface{}, key string) int {
	if val, ok := args[key].(float64); ok {
		return int(val)
	}
	return 0
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

func (s *MCPServer) sendJSONResponse(id interface{}, result interface{}) {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		s.sendToolError(id, fmt.Sprintf("Failed to marshal response: %v", err))
		return
	}

	s.sendResponse(id, ToolResult{
		Content: []ContentItem{{Type: "text", Text: string(data)}},
		IsError: false,
	})
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
	logger.Printf("Tool error: %s\n", msg)
	s.sendResponse(id, ToolResult{
		Content: []ContentItem{{Type: "text", Text: msg}},
		IsError: true,
	})
}
