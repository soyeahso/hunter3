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

// DockerResult is returned from executeDockerCommand as JSON.
type DockerResult struct {
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

func boolProp(desc string) Property {
	return Property{Type: "boolean", Description: desc}
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
	logFile := filepath.Join(logsDir, "mcp-docker.log")
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open log file: %v\n", err)
		return
	}

	// Create logger that writes to both file and stderr
	logger = log.New(io.MultiWriter(f, os.Stderr), "[mcp-docker] ", log.LstdFlags)
	logger.Println("MCP Docker server starting...")
}

func main() {
	initLogger()
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
		ServerInfo:      ServerInfo{Name: "mcp-docker", Version: "1.0.0"},
	})
}

// ---------- Tool definitions ----------

func (s *MCPServer) handleListTools(req JSONRPCRequest) {
	logger.Println("Handling list tools request")

	tools := []Tool{
		// --- Container Management ---
		{
			Name:        "docker_ps",
			Description: "List containers. Supports flags like -a (all), -q (quiet), --filter, --format, etc.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"all":    boolProp("Show all containers (default shows just running)"),
					"quiet":  boolProp("Only display container IDs"),
					"filter": stringArrayProp("Filter output based on conditions (e.g. ['status=running', 'name=myapp'])"),
					"format": stringProp("Format output using a Go template"),
					"flags":  stringArrayProp("Additional flags passed directly to docker ps"),
				},
			},
		},
		{
			Name:        "docker_run",
			Description: "Run a command in a new container. Supports flags like -d (detach), -p (publish ports), -v (volumes), --name, --rm, etc.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"image":       stringProp("Container image to use (e.g. 'nginx:latest', 'ubuntu:22.04')"),
					"command":     stringArrayProp("Command to run in the container (e.g. ['sh', '-c', 'echo hello'])"),
					"detach":      boolProp("Run container in background and print container ID"),
					"name":        stringProp("Assign a name to the container"),
					"ports":       stringArrayProp("Publish container ports (e.g. ['8080:80', '443:443'])"),
					"volumes":     stringArrayProp("Bind mount volumes (e.g. ['/host/path:/container/path'])"),
					"env":         stringArrayProp("Set environment variables (e.g. ['KEY=value', 'DEBUG=1'])"),
					"network":     stringProp("Connect container to a network"),
					"remove":      boolProp("Automatically remove the container when it exits"),
					"interactive": boolProp("Keep STDIN open even if not attached"),
					"tty":         boolProp("Allocate a pseudo-TTY"),
					"flags":       stringArrayProp("Additional flags passed directly to docker run"),
				},
				Required: []string{"image"},
			},
		},
		{
			Name:        "docker_start",
			Description: "Start one or more stopped containers",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"containers": stringArrayProp("Container names or IDs to start"),
					"flags":      stringArrayProp("Additional flags passed directly to docker start"),
				},
				Required: []string{"containers"},
			},
		},
		{
			Name:        "docker_stop",
			Description: "Stop one or more running containers",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"containers": stringArrayProp("Container names or IDs to stop"),
					"time":       stringProp("Seconds to wait before killing the container"),
					"flags":      stringArrayProp("Additional flags passed directly to docker stop"),
				},
				Required: []string{"containers"},
			},
		},
		{
			Name:        "docker_restart",
			Description: "Restart one or more containers",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"containers": stringArrayProp("Container names or IDs to restart"),
					"time":       stringProp("Seconds to wait before killing the container"),
					"flags":      stringArrayProp("Additional flags passed directly to docker restart"),
				},
				Required: []string{"containers"},
			},
		},
		{
			Name:        "docker_rm",
			Description: "Remove one or more containers. Use -f to force remove running containers.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"containers": stringArrayProp("Container names or IDs to remove"),
					"force":      boolProp("Force removal of running containers"),
					"volumes":    boolProp("Remove associated volumes"),
					"flags":      stringArrayProp("Additional flags passed directly to docker rm"),
				},
				Required: []string{"containers"},
			},
		},
		{
			Name:        "docker_exec",
			Description: "Execute a command in a running container",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"container":   stringProp("Container name or ID"),
					"command":     stringArrayProp("Command to execute (e.g. ['sh', '-c', 'ls -la'])"),
					"detach":      boolProp("Detached mode: run command in the background"),
					"interactive": boolProp("Keep STDIN open even if not attached"),
					"tty":         boolProp("Allocate a pseudo-TTY"),
					"user":        stringProp("Username or UID (format: <name|uid>[:<group|gid>])"),
					"workdir":     stringProp("Working directory inside the container"),
					"env":         stringArrayProp("Set environment variables (e.g. ['KEY=value'])"),
					"flags":       stringArrayProp("Additional flags passed directly to docker exec"),
				},
				Required: []string{"container", "command"},
			},
		},
		{
			Name:        "docker_logs",
			Description: "Fetch the logs of a container",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"container":  stringProp("Container name or ID"),
					"follow":     boolProp("Follow log output"),
					"tail":       stringProp("Number of lines to show from the end of the logs (e.g. '100')"),
					"since":      stringProp("Show logs since timestamp (e.g. '2023-01-01T00:00:00')"),
					"until":      stringProp("Show logs before timestamp"),
					"timestamps": boolProp("Show timestamps"),
					"flags":      stringArrayProp("Additional flags passed directly to docker logs"),
				},
				Required: []string{"container"},
			},
		},
		{
			Name:        "docker_inspect",
			Description: "Return low-level information on Docker objects (containers, images, volumes, networks, etc.)",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"objects": stringArrayProp("Objects to inspect (container names/IDs, image names, etc.)"),
					"format":  stringProp("Format output using a Go template"),
					"type":    stringProp("Return JSON for specified type (container, image, volume, network, etc.)"),
					"flags":   stringArrayProp("Additional flags passed directly to docker inspect"),
				},
				Required: []string{"objects"},
			},
		},
		{
			Name:        "docker_stats",
			Description: "Display a live stream of container resource usage statistics",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"containers": stringArrayProp("Container names or IDs (omit for all running containers)"),
					"all":        boolProp("Show all containers (default shows just running)"),
					"no_stream":  boolProp("Disable streaming stats and only pull the first result"),
					"format":     stringProp("Format output using a Go template"),
					"flags":      stringArrayProp("Additional flags passed directly to docker stats"),
				},
			},
		},

		// --- Image Management ---
		{
			Name:        "docker_images",
			Description: "List images. Supports flags like -a (all), -q (quiet), --filter, --format, etc.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"all":    boolProp("Show all images (default hides intermediate images)"),
					"quiet":  boolProp("Only display image IDs"),
					"filter": stringArrayProp("Filter output based on conditions"),
					"format": stringProp("Format output using a Go template"),
					"flags":  stringArrayProp("Additional flags passed directly to docker images"),
				},
			},
		},
		{
			Name:        "docker_pull",
			Description: "Pull an image or a repository from a registry",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"image":    stringProp("Image to pull (e.g. 'nginx:latest', 'ubuntu:22.04')"),
					"all_tags": boolProp("Download all tagged images in the repository"),
					"platform": stringProp("Set platform if server is multi-platform capable (e.g. 'linux/amd64')"),
					"flags":    stringArrayProp("Additional flags passed directly to docker pull"),
				},
				Required: []string{"image"},
			},
		},
		{
			Name:        "docker_push",
			Description: "Push an image or a repository to a registry",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"image":    stringProp("Image to push (e.g. 'myrepo/myimage:tag')"),
					"all_tags": boolProp("Push all tagged images in the repository"),
					"flags":    stringArrayProp("Additional flags passed directly to docker push"),
				},
				Required: []string{"image"},
			},
		},
		{
			Name:        "docker_rmi",
			Description: "Remove one or more images",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"images": stringArrayProp("Image names or IDs to remove"),
					"force":  boolProp("Force removal of the image"),
					"flags":  stringArrayProp("Additional flags passed directly to docker rmi"),
				},
				Required: []string{"images"},
			},
		},
		{
			Name:        "docker_build",
			Description: "Build an image from a Dockerfile",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"path":       stringProp("Build context path (directory containing Dockerfile)"),
					"tag":        stringArrayProp("Name and optionally a tag (e.g. ['myimage:latest', 'myimage:v1.0'])"),
					"file":       stringProp("Name of the Dockerfile (default is 'PATH/Dockerfile')"),
					"build_arg":  stringArrayProp("Set build-time variables (e.g. ['HTTP_PROXY=http://proxy.example.com'])"),
					"no_cache":   boolProp("Do not use cache when building the image"),
					"pull":       boolProp("Always attempt to pull a newer version of the image"),
					"target":     stringProp("Set the target build stage to build"),
					"platform":   stringProp("Set platform if server is multi-platform capable"),
					"label":      stringArrayProp("Set metadata for an image (e.g. ['version=1.0', 'env=prod'])"),
					"network":    stringProp("Set the networking mode for RUN instructions"),
					"flags":      stringArrayProp("Additional flags passed directly to docker build"),
				},
				Required: []string{"path"},
			},
		},
		{
			Name:        "docker_tag",
			Description: "Create a tag TARGET_IMAGE that refers to SOURCE_IMAGE",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"source": stringProp("Source image name or ID"),
					"target": stringProp("Target image name and tag (e.g. 'myrepo/myimage:v1.0')"),
					"flags":  stringArrayProp("Additional flags passed directly to docker tag"),
				},
				Required: []string{"source", "target"},
			},
		},

		// --- Network Management ---
		{
			Name:        "docker_network_ls",
			Description: "List networks",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"quiet":  boolProp("Only display network IDs"),
					"filter": stringArrayProp("Filter output based on conditions"),
					"format": stringProp("Format output using a Go template"),
					"flags":  stringArrayProp("Additional flags passed directly to docker network ls"),
				},
			},
		},
		{
			Name:        "docker_network_create",
			Description: "Create a network",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"name":    stringProp("Network name"),
					"driver":  stringProp("Driver to manage the network (default: bridge)"),
					"subnet":  stringProp("Subnet in CIDR format (e.g. '172.20.0.0/16')"),
					"gateway": stringProp("Gateway for the master subnet"),
					"label":   stringArrayProp("Set metadata on a network (e.g. ['env=prod'])"),
					"flags":   stringArrayProp("Additional flags passed directly to docker network create"),
				},
				Required: []string{"name"},
			},
		},
		{
			Name:        "docker_network_rm",
			Description: "Remove one or more networks",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"networks": stringArrayProp("Network names or IDs to remove"),
					"flags":    stringArrayProp("Additional flags passed directly to docker network rm"),
				},
				Required: []string{"networks"},
			},
		},
		{
			Name:        "docker_network_connect",
			Description: "Connect a container to a network",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"network":   stringProp("Network name or ID"),
					"container": stringProp("Container name or ID"),
					"alias":     stringArrayProp("Add network-scoped alias for the container"),
					"ip":        stringProp("IPv4 address (e.g. '172.20.0.5')"),
					"flags":     stringArrayProp("Additional flags passed directly to docker network connect"),
				},
				Required: []string{"network", "container"},
			},
		},
		{
			Name:        "docker_network_disconnect",
			Description: "Disconnect a container from a network",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"network":   stringProp("Network name or ID"),
					"container": stringProp("Container name or ID"),
					"force":     boolProp("Force the container to disconnect from a network"),
					"flags":     stringArrayProp("Additional flags passed directly to docker network disconnect"),
				},
				Required: []string{"network", "container"},
			},
		},

		// --- Volume Management ---
		{
			Name:        "docker_volume_ls",
			Description: "List volumes",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"quiet":  boolProp("Only display volume names"),
					"filter": stringArrayProp("Filter output based on conditions"),
					"format": stringProp("Format output using a Go template"),
					"flags":  stringArrayProp("Additional flags passed directly to docker volume ls"),
				},
			},
		},
		{
			Name:        "docker_volume_create",
			Description: "Create a volume",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"name":   stringProp("Volume name"),
					"driver": stringProp("Volume driver name (default: local)"),
					"label":  stringArrayProp("Set metadata for a volume (e.g. ['env=prod'])"),
					"opt":    stringArrayProp("Set driver specific options"),
					"flags":  stringArrayProp("Additional flags passed directly to docker volume create"),
				},
			},
		},
		{
			Name:        "docker_volume_rm",
			Description: "Remove one or more volumes",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"volumes": stringArrayProp("Volume names to remove"),
					"force":   boolProp("Force the removal of one or more volumes"),
					"flags":   stringArrayProp("Additional flags passed directly to docker volume rm"),
				},
				Required: []string{"volumes"},
			},
		},
		{
			Name:        "docker_volume_inspect",
			Description: "Display detailed information on one or more volumes",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"volumes": stringArrayProp("Volume names to inspect"),
					"format":  stringProp("Format output using a Go template"),
					"flags":   stringArrayProp("Additional flags passed directly to docker volume inspect"),
				},
				Required: []string{"volumes"},
			},
		},

		// --- Docker Compose (if docker-compose is available) ---
		{
			Name:        "docker_compose_up",
			Description: "Create and start containers defined in docker-compose.yml",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"file":       stringProp("Specify an alternate compose file (default: docker-compose.yml)"),
					"detach":     boolProp("Detached mode: Run containers in the background"),
					"build":      boolProp("Build images before starting containers"),
					"force_recreate": boolProp("Recreate containers even if config/image hasn't changed"),
					"no_build":   boolProp("Don't build an image, even if it's missing"),
					"remove_orphans": boolProp("Remove containers for services not defined in the Compose file"),
					"services":   stringArrayProp("Only start specific services"),
					"flags":      stringArrayProp("Additional flags passed directly to docker-compose up"),
				},
			},
		},
		{
			Name:        "docker_compose_down",
			Description: "Stop and remove containers, networks created by up",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"file":    stringProp("Specify an alternate compose file"),
					"volumes": boolProp("Remove named volumes and anonymous volumes"),
					"rmi":     stringProp("Remove images (type: 'all' or 'local')"),
					"remove_orphans": boolProp("Remove containers for services not defined in the Compose file"),
					"flags":   stringArrayProp("Additional flags passed directly to docker-compose down"),
				},
			},
		},
		{
			Name:        "docker_compose_ps",
			Description: "List containers in compose project",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"file":   stringProp("Specify an alternate compose file"),
					"quiet":  boolProp("Only display container IDs"),
					"all":    boolProp("Show all stopped containers"),
					"format": stringProp("Format output using a Go template"),
					"flags":  stringArrayProp("Additional flags passed directly to docker-compose ps"),
				},
			},
		},
		{
			Name:        "docker_compose_logs",
			Description: "View output from containers in compose project",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"file":       stringProp("Specify an alternate compose file"),
					"follow":     boolProp("Follow log output"),
					"tail":       stringProp("Number of lines to show from the end of the logs"),
					"timestamps": boolProp("Show timestamps"),
					"services":   stringArrayProp("Only show logs for specific services"),
					"flags":      stringArrayProp("Additional flags passed directly to docker-compose logs"),
				},
			},
		},

		// --- System & Info ---
		{
			Name:        "docker_info",
			Description: "Display system-wide information",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"format": stringProp("Format output using a Go template"),
					"flags":  stringArrayProp("Additional flags passed directly to docker info"),
				},
			},
		},
		{
			Name:        "docker_version",
			Description: "Show the Docker version information",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"format": stringProp("Format output using a Go template"),
					"flags":  stringArrayProp("Additional flags passed directly to docker version"),
				},
			},
		},
		{
			Name:        "docker_system_df",
			Description: "Show docker disk usage",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"verbose": boolProp("Show detailed information on space usage"),
					"format":  stringProp("Format output using a Go template"),
					"flags":   stringArrayProp("Additional flags passed directly to docker system df"),
				},
			},
		},
		{
			Name:        "docker_system_prune",
			Description: "Remove unused data (containers, networks, images, build cache)",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"all":     boolProp("Remove all unused images not just dangling ones"),
					"volumes": boolProp("Prune volumes"),
					"force":   boolProp("Do not prompt for confirmation"),
					"filter":  stringArrayProp("Provide filter values (e.g. ['until=24h'])"),
					"flags":   stringArrayProp("Additional flags passed directly to docker system prune"),
				},
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
	// Container commands
	case "docker_ps":
		s.dockerPs(req.ID, args)
	case "docker_run":
		s.dockerRun(req.ID, args)
	case "docker_start":
		s.dockerContainerOp(req.ID, args, "start")
	case "docker_stop":
		s.dockerStopRestart(req.ID, args, "stop")
	case "docker_restart":
		s.dockerStopRestart(req.ID, args, "restart")
	case "docker_rm":
		s.dockerRm(req.ID, args)
	case "docker_exec":
		s.dockerExec(req.ID, args)
	case "docker_logs":
		s.dockerLogs(req.ID, args)
	case "docker_inspect":
		s.dockerInspect(req.ID, args)
	case "docker_stats":
		s.dockerStats(req.ID, args)

	// Image commands
	case "docker_images":
		s.dockerImages(req.ID, args)
	case "docker_pull":
		s.dockerPull(req.ID, args)
	case "docker_push":
		s.dockerPush(req.ID, args)
	case "docker_rmi":
		s.dockerRmi(req.ID, args)
	case "docker_build":
		s.dockerBuild(req.ID, args)
	case "docker_tag":
		s.dockerTag(req.ID, args)

	// Network commands
	case "docker_network_ls":
		s.dockerNetworkLs(req.ID, args)
	case "docker_network_create":
		s.dockerNetworkCreate(req.ID, args)
	case "docker_network_rm":
		s.dockerNetworkRm(req.ID, args)
	case "docker_network_connect":
		s.dockerNetworkConnect(req.ID, args)
	case "docker_network_disconnect":
		s.dockerNetworkDisconnect(req.ID, args)

	// Volume commands
	case "docker_volume_ls":
		s.dockerVolumeLs(req.ID, args)
	case "docker_volume_create":
		s.dockerVolumeCreate(req.ID, args)
	case "docker_volume_rm":
		s.dockerVolumeRm(req.ID, args)
	case "docker_volume_inspect":
		s.dockerVolumeInspect(req.ID, args)

	// Docker Compose commands
	case "docker_compose_up":
		s.dockerComposeUp(req.ID, args)
	case "docker_compose_down":
		s.dockerComposeDown(req.ID, args)
	case "docker_compose_ps":
		s.dockerComposePs(req.ID, args)
	case "docker_compose_logs":
		s.dockerComposeLogs(req.ID, args)

	// System commands
	case "docker_info":
		s.dockerInfo(req.ID, args)
	case "docker_version":
		s.dockerVersion(req.ID, args)
	case "docker_system_df":
		s.dockerSystemDf(req.ID, args)
	case "docker_system_prune":
		s.dockerSystemPrune(req.ID, args)

	default:
		s.sendToolError(req.ID, fmt.Sprintf("Unknown tool: %s", params.Name))
	}
}

// ---------- Container Tool Handlers ----------

func (s *MCPServer) dockerPs(id interface{}, args map[string]interface{}) {
	cmdArgs := []string{"ps"}

	if getBool(args, "all") {
		cmdArgs = append(cmdArgs, "-a")
	}
	if getBool(args, "quiet") {
		cmdArgs = append(cmdArgs, "-q")
	}

	for _, f := range getStringArray(args, "filter") {
		cmdArgs = append(cmdArgs, "--filter", f)
	}

	if format := getString(args, "format"); format != "" {
		cmdArgs = append(cmdArgs, "--format", format)
	}

	cmdArgs = append(cmdArgs, getStringArray(args, "flags")...)
	s.runDocker(id, cmdArgs)
}

func (s *MCPServer) dockerRun(id interface{}, args map[string]interface{}) {
	image := getString(args, "image")
	if image == "" {
		s.sendToolError(id, "image is required")
		return
	}

	cmdArgs := []string{"run"}

	if getBool(args, "detach") {
		cmdArgs = append(cmdArgs, "-d")
	}
	if getBool(args, "remove") {
		cmdArgs = append(cmdArgs, "--rm")
	}
	if getBool(args, "interactive") {
		cmdArgs = append(cmdArgs, "-i")
	}
	if getBool(args, "tty") {
		cmdArgs = append(cmdArgs, "-t")
	}

	if name := getString(args, "name"); name != "" {
		cmdArgs = append(cmdArgs, "--name", name)
	}
	if network := getString(args, "network"); network != "" {
		cmdArgs = append(cmdArgs, "--network", network)
	}

	for _, port := range getStringArray(args, "ports") {
		cmdArgs = append(cmdArgs, "-p", port)
	}
	for _, vol := range getStringArray(args, "volumes") {
		cmdArgs = append(cmdArgs, "-v", vol)
	}
	for _, env := range getStringArray(args, "env") {
		cmdArgs = append(cmdArgs, "-e", env)
	}

	cmdArgs = append(cmdArgs, getStringArray(args, "flags")...)
	cmdArgs = append(cmdArgs, image)
	cmdArgs = append(cmdArgs, getStringArray(args, "command")...)

	s.runDocker(id, cmdArgs)
}

func (s *MCPServer) dockerContainerOp(id interface{}, args map[string]interface{}, op string) {
	containers := getStringArray(args, "containers")
	if len(containers) == 0 {
		s.sendToolError(id, "containers is required")
		return
	}

	cmdArgs := []string{op}
	cmdArgs = append(cmdArgs, getStringArray(args, "flags")...)
	cmdArgs = append(cmdArgs, containers...)

	s.runDocker(id, cmdArgs)
}

func (s *MCPServer) dockerStopRestart(id interface{}, args map[string]interface{}, op string) {
	containers := getStringArray(args, "containers")
	if len(containers) == 0 {
		s.sendToolError(id, "containers is required")
		return
	}

	cmdArgs := []string{op}

	if time := getString(args, "time"); time != "" {
		cmdArgs = append(cmdArgs, "-t", time)
	}

	cmdArgs = append(cmdArgs, getStringArray(args, "flags")...)
	cmdArgs = append(cmdArgs, containers...)

	s.runDocker(id, cmdArgs)
}

func (s *MCPServer) dockerRm(id interface{}, args map[string]interface{}) {
	containers := getStringArray(args, "containers")
	if len(containers) == 0 {
		s.sendToolError(id, "containers is required")
		return
	}

	cmdArgs := []string{"rm"}

	if getBool(args, "force") {
		cmdArgs = append(cmdArgs, "-f")
	}
	if getBool(args, "volumes") {
		cmdArgs = append(cmdArgs, "-v")
	}

	cmdArgs = append(cmdArgs, getStringArray(args, "flags")...)
	cmdArgs = append(cmdArgs, containers...)

	s.runDocker(id, cmdArgs)
}

func (s *MCPServer) dockerExec(id interface{}, args map[string]interface{}) {
	container := getString(args, "container")
	command := getStringArray(args, "command")
	if container == "" || len(command) == 0 {
		s.sendToolError(id, "container and command are required")
		return
	}

	cmdArgs := []string{"exec"}

	if getBool(args, "detach") {
		cmdArgs = append(cmdArgs, "-d")
	}
	if getBool(args, "interactive") {
		cmdArgs = append(cmdArgs, "-i")
	}
	if getBool(args, "tty") {
		cmdArgs = append(cmdArgs, "-t")
	}

	if user := getString(args, "user"); user != "" {
		cmdArgs = append(cmdArgs, "-u", user)
	}
	if workdir := getString(args, "workdir"); workdir != "" {
		cmdArgs = append(cmdArgs, "-w", workdir)
	}

	for _, env := range getStringArray(args, "env") {
		cmdArgs = append(cmdArgs, "-e", env)
	}

	cmdArgs = append(cmdArgs, getStringArray(args, "flags")...)
	cmdArgs = append(cmdArgs, container)
	cmdArgs = append(cmdArgs, command...)

	s.runDocker(id, cmdArgs)
}

func (s *MCPServer) dockerLogs(id interface{}, args map[string]interface{}) {
	container := getString(args, "container")
	if container == "" {
		s.sendToolError(id, "container is required")
		return
	}

	cmdArgs := []string{"logs"}

	if getBool(args, "follow") {
		cmdArgs = append(cmdArgs, "-f")
	}
	if getBool(args, "timestamps") {
		cmdArgs = append(cmdArgs, "-t")
	}

	if tail := getString(args, "tail"); tail != "" {
		cmdArgs = append(cmdArgs, "--tail", tail)
	}
	if since := getString(args, "since"); since != "" {
		cmdArgs = append(cmdArgs, "--since", since)
	}
	if until := getString(args, "until"); until != "" {
		cmdArgs = append(cmdArgs, "--until", until)
	}

	cmdArgs = append(cmdArgs, getStringArray(args, "flags")...)
	cmdArgs = append(cmdArgs, container)

	s.runDocker(id, cmdArgs)
}

func (s *MCPServer) dockerInspect(id interface{}, args map[string]interface{}) {
	objects := getStringArray(args, "objects")
	if len(objects) == 0 {
		s.sendToolError(id, "objects is required")
		return
	}

	cmdArgs := []string{"inspect"}

	if format := getString(args, "format"); format != "" {
		cmdArgs = append(cmdArgs, "--format", format)
	}
	if typ := getString(args, "type"); typ != "" {
		cmdArgs = append(cmdArgs, "--type", typ)
	}

	cmdArgs = append(cmdArgs, getStringArray(args, "flags")...)
	cmdArgs = append(cmdArgs, objects...)

	s.runDocker(id, cmdArgs)
}

func (s *MCPServer) dockerStats(id interface{}, args map[string]interface{}) {
	cmdArgs := []string{"stats"}

	if getBool(args, "all") {
		cmdArgs = append(cmdArgs, "-a")
	}
	if getBool(args, "no_stream") {
		cmdArgs = append(cmdArgs, "--no-stream")
	}

	if format := getString(args, "format"); format != "" {
		cmdArgs = append(cmdArgs, "--format", format)
	}

	cmdArgs = append(cmdArgs, getStringArray(args, "flags")...)
	cmdArgs = append(cmdArgs, getStringArray(args, "containers")...)

	s.runDocker(id, cmdArgs)
}

// ---------- Image Tool Handlers ----------

func (s *MCPServer) dockerImages(id interface{}, args map[string]interface{}) {
	cmdArgs := []string{"images"}

	if getBool(args, "all") {
		cmdArgs = append(cmdArgs, "-a")
	}
	if getBool(args, "quiet") {
		cmdArgs = append(cmdArgs, "-q")
	}

	for _, f := range getStringArray(args, "filter") {
		cmdArgs = append(cmdArgs, "--filter", f)
	}

	if format := getString(args, "format"); format != "" {
		cmdArgs = append(cmdArgs, "--format", format)
	}

	cmdArgs = append(cmdArgs, getStringArray(args, "flags")...)
	s.runDocker(id, cmdArgs)
}

func (s *MCPServer) dockerPull(id interface{}, args map[string]interface{}) {
	image := getString(args, "image")
	if image == "" {
		s.sendToolError(id, "image is required")
		return
	}

	cmdArgs := []string{"pull"}

	if getBool(args, "all_tags") {
		cmdArgs = append(cmdArgs, "-a")
	}
	if platform := getString(args, "platform"); platform != "" {
		cmdArgs = append(cmdArgs, "--platform", platform)
	}

	cmdArgs = append(cmdArgs, getStringArray(args, "flags")...)
	cmdArgs = append(cmdArgs, image)

	s.runDocker(id, cmdArgs)
}

func (s *MCPServer) dockerPush(id interface{}, args map[string]interface{}) {
	image := getString(args, "image")
	if image == "" {
		s.sendToolError(id, "image is required")
		return
	}

	cmdArgs := []string{"push"}

	if getBool(args, "all_tags") {
		cmdArgs = append(cmdArgs, "-a")
	}

	cmdArgs = append(cmdArgs, getStringArray(args, "flags")...)
	cmdArgs = append(cmdArgs, image)

	s.runDocker(id, cmdArgs)
}

func (s *MCPServer) dockerRmi(id interface{}, args map[string]interface{}) {
	images := getStringArray(args, "images")
	if len(images) == 0 {
		s.sendToolError(id, "images is required")
		return
	}

	cmdArgs := []string{"rmi"}

	if getBool(args, "force") {
		cmdArgs = append(cmdArgs, "-f")
	}

	cmdArgs = append(cmdArgs, getStringArray(args, "flags")...)
	cmdArgs = append(cmdArgs, images...)

	s.runDocker(id, cmdArgs)
}

func (s *MCPServer) dockerBuild(id interface{}, args map[string]interface{}) {
	path := getString(args, "path")
	if path == "" {
		s.sendToolError(id, "path is required")
		return
	}

	cmdArgs := []string{"build"}

	for _, tag := range getStringArray(args, "tag") {
		cmdArgs = append(cmdArgs, "-t", tag)
	}

	if file := getString(args, "file"); file != "" {
		cmdArgs = append(cmdArgs, "-f", file)
	}

	for _, arg := range getStringArray(args, "build_arg") {
		cmdArgs = append(cmdArgs, "--build-arg", arg)
	}

	for _, label := range getStringArray(args, "label") {
		cmdArgs = append(cmdArgs, "--label", label)
	}

	if getBool(args, "no_cache") {
		cmdArgs = append(cmdArgs, "--no-cache")
	}
	if getBool(args, "pull") {
		cmdArgs = append(cmdArgs, "--pull")
	}

	if target := getString(args, "target"); target != "" {
		cmdArgs = append(cmdArgs, "--target", target)
	}
	if platform := getString(args, "platform"); platform != "" {
		cmdArgs = append(cmdArgs, "--platform", platform)
	}
	if network := getString(args, "network"); network != "" {
		cmdArgs = append(cmdArgs, "--network", network)
	}

	cmdArgs = append(cmdArgs, getStringArray(args, "flags")...)
	cmdArgs = append(cmdArgs, path)

	s.runDocker(id, cmdArgs)
}

func (s *MCPServer) dockerTag(id interface{}, args map[string]interface{}) {
	source := getString(args, "source")
	target := getString(args, "target")
	if source == "" || target == "" {
		s.sendToolError(id, "source and target are required")
		return
	}

	cmdArgs := []string{"tag"}
	cmdArgs = append(cmdArgs, getStringArray(args, "flags")...)
	cmdArgs = append(cmdArgs, source, target)

	s.runDocker(id, cmdArgs)
}

// ---------- Network Tool Handlers ----------

func (s *MCPServer) dockerNetworkLs(id interface{}, args map[string]interface{}) {
	cmdArgs := []string{"network", "ls"}

	if getBool(args, "quiet") {
		cmdArgs = append(cmdArgs, "-q")
	}

	for _, f := range getStringArray(args, "filter") {
		cmdArgs = append(cmdArgs, "--filter", f)
	}

	if format := getString(args, "format"); format != "" {
		cmdArgs = append(cmdArgs, "--format", format)
	}

	cmdArgs = append(cmdArgs, getStringArray(args, "flags")...)
	s.runDocker(id, cmdArgs)
}

func (s *MCPServer) dockerNetworkCreate(id interface{}, args map[string]interface{}) {
	name := getString(args, "name")
	if name == "" {
		s.sendToolError(id, "name is required")
		return
	}

	cmdArgs := []string{"network", "create"}

	if driver := getString(args, "driver"); driver != "" {
		cmdArgs = append(cmdArgs, "--driver", driver)
	}
	if subnet := getString(args, "subnet"); subnet != "" {
		cmdArgs = append(cmdArgs, "--subnet", subnet)
	}
	if gateway := getString(args, "gateway"); gateway != "" {
		cmdArgs = append(cmdArgs, "--gateway", gateway)
	}

	for _, label := range getStringArray(args, "label") {
		cmdArgs = append(cmdArgs, "--label", label)
	}

	cmdArgs = append(cmdArgs, getStringArray(args, "flags")...)
	cmdArgs = append(cmdArgs, name)

	s.runDocker(id, cmdArgs)
}

func (s *MCPServer) dockerNetworkRm(id interface{}, args map[string]interface{}) {
	networks := getStringArray(args, "networks")
	if len(networks) == 0 {
		s.sendToolError(id, "networks is required")
		return
	}

	cmdArgs := []string{"network", "rm"}
	cmdArgs = append(cmdArgs, getStringArray(args, "flags")...)
	cmdArgs = append(cmdArgs, networks...)

	s.runDocker(id, cmdArgs)
}

func (s *MCPServer) dockerNetworkConnect(id interface{}, args map[string]interface{}) {
	network := getString(args, "network")
	container := getString(args, "container")
	if network == "" || container == "" {
		s.sendToolError(id, "network and container are required")
		return
	}

	cmdArgs := []string{"network", "connect"}

	for _, alias := range getStringArray(args, "alias") {
		cmdArgs = append(cmdArgs, "--alias", alias)
	}
	if ip := getString(args, "ip"); ip != "" {
		cmdArgs = append(cmdArgs, "--ip", ip)
	}

	cmdArgs = append(cmdArgs, getStringArray(args, "flags")...)
	cmdArgs = append(cmdArgs, network, container)

	s.runDocker(id, cmdArgs)
}

func (s *MCPServer) dockerNetworkDisconnect(id interface{}, args map[string]interface{}) {
	network := getString(args, "network")
	container := getString(args, "container")
	if network == "" || container == "" {
		s.sendToolError(id, "network and container are required")
		return
	}

	cmdArgs := []string{"network", "disconnect"}

	if getBool(args, "force") {
		cmdArgs = append(cmdArgs, "-f")
	}

	cmdArgs = append(cmdArgs, getStringArray(args, "flags")...)
	cmdArgs = append(cmdArgs, network, container)

	s.runDocker(id, cmdArgs)
}

// ---------- Volume Tool Handlers ----------

func (s *MCPServer) dockerVolumeLs(id interface{}, args map[string]interface{}) {
	cmdArgs := []string{"volume", "ls"}

	if getBool(args, "quiet") {
		cmdArgs = append(cmdArgs, "-q")
	}

	for _, f := range getStringArray(args, "filter") {
		cmdArgs = append(cmdArgs, "--filter", f)
	}

	if format := getString(args, "format"); format != "" {
		cmdArgs = append(cmdArgs, "--format", format)
	}

	cmdArgs = append(cmdArgs, getStringArray(args, "flags")...)
	s.runDocker(id, cmdArgs)
}

func (s *MCPServer) dockerVolumeCreate(id interface{}, args map[string]interface{}) {
	cmdArgs := []string{"volume", "create"}

	if name := getString(args, "name"); name != "" {
		cmdArgs = append(cmdArgs, name)
	}

	if driver := getString(args, "driver"); driver != "" {
		cmdArgs = append(cmdArgs, "--driver", driver)
	}

	for _, label := range getStringArray(args, "label") {
		cmdArgs = append(cmdArgs, "--label", label)
	}
	for _, opt := range getStringArray(args, "opt") {
		cmdArgs = append(cmdArgs, "--opt", opt)
	}

	cmdArgs = append(cmdArgs, getStringArray(args, "flags")...)
	s.runDocker(id, cmdArgs)
}

func (s *MCPServer) dockerVolumeRm(id interface{}, args map[string]interface{}) {
	volumes := getStringArray(args, "volumes")
	if len(volumes) == 0 {
		s.sendToolError(id, "volumes is required")
		return
	}

	cmdArgs := []string{"volume", "rm"}

	if getBool(args, "force") {
		cmdArgs = append(cmdArgs, "-f")
	}

	cmdArgs = append(cmdArgs, getStringArray(args, "flags")...)
	cmdArgs = append(cmdArgs, volumes...)

	s.runDocker(id, cmdArgs)
}

func (s *MCPServer) dockerVolumeInspect(id interface{}, args map[string]interface{}) {
	volumes := getStringArray(args, "volumes")
	if len(volumes) == 0 {
		s.sendToolError(id, "volumes is required")
		return
	}

	cmdArgs := []string{"volume", "inspect"}

	if format := getString(args, "format"); format != "" {
		cmdArgs = append(cmdArgs, "--format", format)
	}

	cmdArgs = append(cmdArgs, getStringArray(args, "flags")...)
	cmdArgs = append(cmdArgs, volumes...)

	s.runDocker(id, cmdArgs)
}

// ---------- Docker Compose Tool Handlers ----------

func (s *MCPServer) dockerComposeUp(id interface{}, args map[string]interface{}) {
	cmdArgs := []string{"compose"}

	if file := getString(args, "file"); file != "" {
		cmdArgs = append(cmdArgs, "-f", file)
	}

	cmdArgs = append(cmdArgs, "up")

	if getBool(args, "detach") {
		cmdArgs = append(cmdArgs, "-d")
	}
	if getBool(args, "build") {
		cmdArgs = append(cmdArgs, "--build")
	}
	if getBool(args, "force_recreate") {
		cmdArgs = append(cmdArgs, "--force-recreate")
	}
	if getBool(args, "no_build") {
		cmdArgs = append(cmdArgs, "--no-build")
	}
	if getBool(args, "remove_orphans") {
		cmdArgs = append(cmdArgs, "--remove-orphans")
	}

	cmdArgs = append(cmdArgs, getStringArray(args, "flags")...)
	cmdArgs = append(cmdArgs, getStringArray(args, "services")...)

	s.runDocker(id, cmdArgs)
}

func (s *MCPServer) dockerComposeDown(id interface{}, args map[string]interface{}) {
	cmdArgs := []string{"compose"}

	if file := getString(args, "file"); file != "" {
		cmdArgs = append(cmdArgs, "-f", file)
	}

	cmdArgs = append(cmdArgs, "down")

	if getBool(args, "volumes") {
		cmdArgs = append(cmdArgs, "-v")
	}
	if getBool(args, "remove_orphans") {
		cmdArgs = append(cmdArgs, "--remove-orphans")
	}
	if rmi := getString(args, "rmi"); rmi != "" {
		cmdArgs = append(cmdArgs, "--rmi", rmi)
	}

	cmdArgs = append(cmdArgs, getStringArray(args, "flags")...)
	s.runDocker(id, cmdArgs)
}

func (s *MCPServer) dockerComposePs(id interface{}, args map[string]interface{}) {
	cmdArgs := []string{"compose"}

	if file := getString(args, "file"); file != "" {
		cmdArgs = append(cmdArgs, "-f", file)
	}

	cmdArgs = append(cmdArgs, "ps")

	if getBool(args, "quiet") {
		cmdArgs = append(cmdArgs, "-q")
	}
	if getBool(args, "all") {
		cmdArgs = append(cmdArgs, "-a")
	}
	if format := getString(args, "format"); format != "" {
		cmdArgs = append(cmdArgs, "--format", format)
	}

	cmdArgs = append(cmdArgs, getStringArray(args, "flags")...)
	s.runDocker(id, cmdArgs)
}

func (s *MCPServer) dockerComposeLogs(id interface{}, args map[string]interface{}) {
	cmdArgs := []string{"compose"}

	if file := getString(args, "file"); file != "" {
		cmdArgs = append(cmdArgs, "-f", file)
	}

	cmdArgs = append(cmdArgs, "logs")

	if getBool(args, "follow") {
		cmdArgs = append(cmdArgs, "-f")
	}
	if getBool(args, "timestamps") {
		cmdArgs = append(cmdArgs, "-t")
	}
	if tail := getString(args, "tail"); tail != "" {
		cmdArgs = append(cmdArgs, "--tail", tail)
	}

	cmdArgs = append(cmdArgs, getStringArray(args, "flags")...)
	cmdArgs = append(cmdArgs, getStringArray(args, "services")...)

	s.runDocker(id, cmdArgs)
}

// ---------- System Tool Handlers ----------

func (s *MCPServer) dockerInfo(id interface{}, args map[string]interface{}) {
	cmdArgs := []string{"info"}

	if format := getString(args, "format"); format != "" {
		cmdArgs = append(cmdArgs, "--format", format)
	}

	cmdArgs = append(cmdArgs, getStringArray(args, "flags")...)
	s.runDocker(id, cmdArgs)
}

func (s *MCPServer) dockerVersion(id interface{}, args map[string]interface{}) {
	cmdArgs := []string{"version"}

	if format := getString(args, "format"); format != "" {
		cmdArgs = append(cmdArgs, "--format", format)
	}

	cmdArgs = append(cmdArgs, getStringArray(args, "flags")...)
	s.runDocker(id, cmdArgs)
}

func (s *MCPServer) dockerSystemDf(id interface{}, args map[string]interface{}) {
	cmdArgs := []string{"system", "df"}

	if getBool(args, "verbose") {
		cmdArgs = append(cmdArgs, "-v")
	}
	if format := getString(args, "format"); format != "" {
		cmdArgs = append(cmdArgs, "--format", format)
	}

	cmdArgs = append(cmdArgs, getStringArray(args, "flags")...)
	s.runDocker(id, cmdArgs)
}

func (s *MCPServer) dockerSystemPrune(id interface{}, args map[string]interface{}) {
	cmdArgs := []string{"system", "prune"}

	if getBool(args, "all") {
		cmdArgs = append(cmdArgs, "-a")
	}
	if getBool(args, "volumes") {
		cmdArgs = append(cmdArgs, "--volumes")
	}
	if getBool(args, "force") {
		cmdArgs = append(cmdArgs, "-f")
	}

	for _, f := range getStringArray(args, "filter") {
		cmdArgs = append(cmdArgs, "--filter", f)
	}

	cmdArgs = append(cmdArgs, getStringArray(args, "flags")...)
	s.runDocker(id, cmdArgs)
}

// ---------- Docker execution ----------

func (s *MCPServer) runDocker(id interface{}, dockerArgs []string) {
	cmd := exec.Command("docker", dockerArgs...)

	commandStr := "docker " + strings.Join(dockerArgs, " ")
	logger.Printf("Executing: %s\n", commandStr)

	stdout, err := cmd.Output()
	result := DockerResult{
		Command: commandStr,
		Success: err == nil,
		Stdout:  strings.TrimSpace(string(stdout)),
	}

	if err != nil {
		logger.Printf("Docker command failed: %v\n", err)
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.Stderr = strings.TrimSpace(string(exitErr.Stderr))
			logger.Printf("Docker stderr: %s\n", result.Stderr)
		}
		result.Error = err.Error()
	} else {
		logger.Printf("Docker command succeeded, stdout length: %d bytes\n", len(result.Stdout))
	}

	data, _ := json.MarshalIndent(result, "", "  ")
	s.sendResponse(id, ToolResult{
		Content: []ContentItem{{Type: "text", Text: string(data)}},
		IsError: !result.Success,
	})
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
