package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

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
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Enum        []string `json:"enum,omitempty"`
	Default     string   `json:"default,omitempty"`
	Minimum     *float64 `json:"minimum,omitempty"`
	Maximum     *float64 `json:"maximum,omitempty"`
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

// NOAA API Response Types
type PointsResponse struct {
	Properties PointsProperties `json:"properties"`
}

type PointsProperties struct {
	Forecast       string `json:"forecast"`
	ForecastHourly string `json:"forecastHourly"`
	GridID         string `json:"gridId"`
	GridX          int    `json:"gridX"`
	GridY          int    `json:"gridY"`
	ObservationStations string `json:"observationStations"`
}

type ForecastResponse struct {
	Properties ForecastProperties `json:"properties"`
}

type ForecastProperties struct {
	Updated string   `json:"updated"`
	Periods []Period `json:"periods"`
}

type Period struct {
	Number           int     `json:"number"`
	Name             string  `json:"name"`
	StartTime        string  `json:"startTime"`
	EndTime          string  `json:"endTime"`
	IsDaytime        bool    `json:"isDaytime"`
	Temperature      int     `json:"temperature"`
	TemperatureUnit  string  `json:"temperatureUnit"`
	TemperatureTrend string  `json:"temperatureTrend,omitempty"`
	WindSpeed        string  `json:"windSpeed"`
	WindDirection    string  `json:"windDirection"`
	Icon             string  `json:"icon"`
	ShortForecast    string  `json:"shortForecast"`
	DetailedForecast string  `json:"detailedForecast"`
}

type AlertsResponse struct {
	Features []AlertFeature `json:"features"`
}

type AlertFeature struct {
	Properties AlertProperties `json:"properties"`
}

type AlertProperties struct {
	Event       string   `json:"event"`
	Headline    string   `json:"headline"`
	Description string   `json:"description"`
	Severity    string   `json:"severity"`
	Certainty   string   `json:"certainty"`
	Urgency     string   `json:"urgency"`
	AreaDesc    string   `json:"areaDesc"`
	Onset       string   `json:"onset"`
	Expires     string   `json:"expires"`
	Instruction string   `json:"instruction"`
}

type StationsResponse struct {
	Features []StationFeature `json:"features"`
}

type StationFeature struct {
	Properties StationProperties `json:"properties"`
}

type StationProperties struct {
	StationIdentifier string `json:"stationIdentifier"`
	Name              string `json:"name"`
}

type ObservationResponse struct {
	Properties ObservationProperties `json:"properties"`
}

type ObservationProperties struct {
	Timestamp           string               `json:"timestamp"`
	TextDescription     string               `json:"textDescription"`
	Temperature         ValueWithUnit        `json:"temperature"`
	Dewpoint            ValueWithUnit        `json:"dewpoint"`
	WindDirection       ValueWithUnit        `json:"windDirection"`
	WindSpeed           ValueWithUnit        `json:"windSpeed"`
	WindGust            ValueWithUnit        `json:"windGust"`
	BarometricPressure  ValueWithUnit        `json:"barometricPressure"`
	RelativeHumidity    ValueWithUnit        `json:"relativeHumidity"`
	Visibility          ValueWithUnit        `json:"visibility"`
	PrecipitationLastHour ValueWithUnit      `json:"precipitationLastHour"`
}

type ValueWithUnit struct {
	Value       *float64 `json:"value"`
	UnitCode    string   `json:"unitCode"`
	QualityControl string `json:"qualityControl,omitempty"`
}

var logger *log.Logger

func initLogger() {
	// Create logs directory if it doesn't exist
	logsDir := "/home/genoeg/.hunter3/logs"
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create logs directory: %v\n", err)
		return
	}

	// Open log file
	logFile := filepath.Join(logsDir, "mcp-weather.log")
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open log file: %v\n", err)
		return
	}

	// Create logger that writes to both file and stderr
	logger = log.New(io.MultiWriter(f, os.Stderr), "[mcp-weather] ", log.LstdFlags)
	logger.Println("MCP Weather server starting...")
}

func main() {
	initLogger()

	server := &MCPServer{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
	logger.Println("Server initialized")
	server.Run()
}

type MCPServer struct {
	httpClient *http.Client
}

func (s *MCPServer) Run() {
	scanner := bufio.NewScanner(os.Stdin)

	// Increase buffer size for large inputs
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
		// Ignore this notification
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
			Name:    "weather",
			Version: "1.0.0",
		},
	}

	s.sendResponse(req.ID, result)
}

func (s *MCPServer) handleListTools(req JSONRPCRequest) {
	logger.Println("Handling list tools request")
	
	minLat := -90.0
	maxLat := 90.0
	minLon := -180.0
	maxLon := 180.0
	
	tools := []Tool{
		{
			Name:        "get_forecast",
			Description: "Get weather forecast for a location using latitude and longitude. Provides 12-hour period forecasts for the next 7 days from NOAA/NWS.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"latitude": {
						Type:        "number",
						Description: "Latitude of the location (-90 to 90)",
						Minimum:     &minLat,
						Maximum:     &maxLat,
					},
					"longitude": {
						Type:        "number",
						Description: "Longitude of the location (-180 to 180)",
						Minimum:     &minLon,
						Maximum:     &maxLon,
					},
					"hourly": {
						Type:        "string",
						Description: "Get hourly forecast instead of 12-hour periods",
						Enum:        []string{"true", "false"},
						Default:     "false",
					},
				},
				Required: []string{"latitude", "longitude"},
			},
		},
		{
			Name:        "get_alerts",
			Description: "Get active weather alerts for a US state or specific area. Returns NWS weather warnings, watches, and advisories.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"state": {
						Type:        "string",
						Description: "Two-letter US state code (e.g., 'KS', 'CA', 'NY'). Required if area is not specified.",
					},
					"area": {
						Type:        "string",
						Description: "Specific area code (zone or county). Optional - if not specified, returns all alerts for the state.",
					},
				},
				Required: []string{},
			},
		},
		{
			Name:        "get_observation",
			Description: "Get current weather observations for a location using latitude and longitude. Provides real-time weather conditions from the nearest NOAA weather station.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"latitude": {
						Type:        "number",
						Description: "Latitude of the location (-90 to 90)",
						Minimum:     &minLat,
						Maximum:     &maxLat,
					},
					"longitude": {
						Type:        "number",
						Description: "Longitude of the location (-180 to 180)",
						Minimum:     &minLon,
						Maximum:     &maxLon,
					},
				},
				Required: []string{"latitude", "longitude"},
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
	case "get_forecast":
		s.getForecast(req.ID, params.Arguments)
	case "get_alerts":
		s.getAlerts(req.ID, params.Arguments)
	case "get_observation":
		s.getObservation(req.ID, params.Arguments)
	default:
		logger.Printf("Unknown tool: %s\n", params.Name)
		s.sendError(req.ID, -32602, "Unknown tool", fmt.Sprintf("Tool not found: %s", params.Name))
	}
}

func (s *MCPServer) getForecast(id interface{}, args map[string]interface{}) {
	lat, latOk := args["latitude"].(float64)
	lon, lonOk := args["longitude"].(float64)
	
	if !latOk || !lonOk {
		s.sendError(id, -32602, "Invalid arguments", "latitude and longitude are required as numbers")
		return
	}

	hourly := false
	if h, ok := args["hourly"].(string); ok && h == "true" {
		hourly = true
	}

	logger.Printf("Getting forecast for lat=%f, lon=%f, hourly=%v\n", lat, lon, hourly)

	// Step 1: Get grid point information
	pointsURL := fmt.Sprintf("https://api.weather.gov/points/%f,%f", lat, lon)
	pointsResp, err := s.makeRequest(pointsURL)
	if err != nil {
		s.sendToolError(id, fmt.Sprintf("Failed to get grid point information: %v", err))
		return
	}

	var pointsData PointsResponse
	if err := json.Unmarshal(pointsResp, &pointsData); err != nil {
		s.sendToolError(id, fmt.Sprintf("Failed to parse grid point data: %v", err))
		return
	}

	// Step 2: Get forecast
	forecastURL := pointsData.Properties.Forecast
	if hourly {
		forecastURL = pointsData.Properties.ForecastHourly
	}

	logger.Printf("Fetching forecast from: %s\n", forecastURL)
	forecastResp, err := s.makeRequest(forecastURL)
	if err != nil {
		s.sendToolError(id, fmt.Sprintf("Failed to get forecast: %v", err))
		return
	}

	var forecastData ForecastResponse
	if err := json.Unmarshal(forecastResp, &forecastData); err != nil {
		s.sendToolError(id, fmt.Sprintf("Failed to parse forecast data: %v", err))
		return
	}

	// Format the forecast
	var output string
	output += fmt.Sprintf("Weather Forecast for %.4f, %.4f\n", lat, lon)
	output += fmt.Sprintf("Updated: %s\n\n", forecastData.Properties.Updated)

	for _, period := range forecastData.Properties.Periods {
		output += fmt.Sprintf("=== %s ===\n", period.Name)
		output += fmt.Sprintf("Temperature: %d°%s", period.Temperature, period.TemperatureUnit)
		if period.TemperatureTrend != "" {
			output += fmt.Sprintf(" (trend: %s)", period.TemperatureTrend)
		}
		output += "\n"
		output += fmt.Sprintf("Wind: %s at %s\n", period.WindDirection, period.WindSpeed)
		output += fmt.Sprintf("Short Forecast: %s\n", period.ShortForecast)
		output += fmt.Sprintf("Detailed: %s\n\n", period.DetailedForecast)
	}

	result := ToolResult{
		Content: []ContentItem{
			{
				Type: "text",
				Text: output,
			},
		},
	}

	s.sendResponse(id, result)
}

func (s *MCPServer) getAlerts(id interface{}, args map[string]interface{}) {
	state, stateOk := args["state"].(string)
	area, _ := args["area"].(string)

	if !stateOk || state == "" {
		s.sendError(id, -32602, "Invalid arguments", "state parameter is required")
		return
	}

	logger.Printf("Getting alerts for state=%s, area=%s\n", state, area)

	// Build alerts URL
	alertsURL := fmt.Sprintf("https://api.weather.gov/alerts/active?area=%s", state)
	if area != "" {
		alertsURL += fmt.Sprintf("&zone=%s", area)
	}

	logger.Printf("Fetching alerts from: %s\n", alertsURL)
	alertsResp, err := s.makeRequest(alertsURL)
	if err != nil {
		s.sendToolError(id, fmt.Sprintf("Failed to get alerts: %v", err))
		return
	}

	var alertsData AlertsResponse
	if err := json.Unmarshal(alertsResp, &alertsData); err != nil {
		s.sendToolError(id, fmt.Sprintf("Failed to parse alerts data: %v", err))
		return
	}

	// Format the alerts
	var output string
	if len(alertsData.Features) == 0 {
		output = fmt.Sprintf("No active weather alerts for %s\n", state)
	} else {
		output = fmt.Sprintf("Active Weather Alerts for %s (%d alerts)\n\n", state, len(alertsData.Features))
		
		for i, alert := range alertsData.Features {
			props := alert.Properties
			output += fmt.Sprintf("=== Alert %d: %s ===\n", i+1, props.Event)
			output += fmt.Sprintf("Severity: %s | Certainty: %s | Urgency: %s\n", props.Severity, props.Certainty, props.Urgency)
			output += fmt.Sprintf("Area: %s\n", props.AreaDesc)
			output += fmt.Sprintf("Onset: %s\n", props.Onset)
			output += fmt.Sprintf("Expires: %s\n", props.Expires)
			output += fmt.Sprintf("Headline: %s\n", props.Headline)
			output += fmt.Sprintf("\nDescription:\n%s\n", props.Description)
			if props.Instruction != "" {
				output += fmt.Sprintf("\nInstructions:\n%s\n", props.Instruction)
			}
			output += "\n"
		}
	}

	result := ToolResult{
		Content: []ContentItem{
			{
				Type: "text",
				Text: output,
			},
		},
	}

	s.sendResponse(id, result)
}

func (s *MCPServer) getObservation(id interface{}, args map[string]interface{}) {
	lat, latOk := args["latitude"].(float64)
	lon, lonOk := args["longitude"].(float64)
	
	if !latOk || !lonOk {
		s.sendError(id, -32602, "Invalid arguments", "latitude and longitude are required as numbers")
		return
	}

	logger.Printf("Getting observation for lat=%f, lon=%f\n", lat, lon)

	// Step 1: Get grid point information
	pointsURL := fmt.Sprintf("https://api.weather.gov/points/%f,%f", lat, lon)
	pointsResp, err := s.makeRequest(pointsURL)
	if err != nil {
		s.sendToolError(id, fmt.Sprintf("Failed to get grid point information: %v", err))
		return
	}

	var pointsData PointsResponse
	if err := json.Unmarshal(pointsResp, &pointsData); err != nil {
		s.sendToolError(id, fmt.Sprintf("Failed to parse grid point data: %v", err))
		return
	}

	// Step 2: Get observation stations
	stationsURL := pointsData.Properties.ObservationStations
	logger.Printf("Fetching stations from: %s\n", stationsURL)
	stationsResp, err := s.makeRequest(stationsURL)
	if err != nil {
		s.sendToolError(id, fmt.Sprintf("Failed to get observation stations: %v", err))
		return
	}

	var stationsData StationsResponse
	if err := json.Unmarshal(stationsResp, &stationsData); err != nil {
		s.sendToolError(id, fmt.Sprintf("Failed to parse stations data: %v", err))
		return
	}

	if len(stationsData.Features) == 0 {
		s.sendToolError(id, "No observation stations found near this location")
		return
	}

	// Step 3: Get latest observation from nearest station
	stationID := stationsData.Features[0].Properties.StationIdentifier
	observationURL := fmt.Sprintf("https://api.weather.gov/stations/%s/observations/latest", stationID)
	logger.Printf("Fetching observation from: %s\n", observationURL)
	obsResp, err := s.makeRequest(observationURL)
	if err != nil {
		s.sendToolError(id, fmt.Sprintf("Failed to get observation: %v", err))
		return
	}

	var obsData ObservationResponse
	if err := json.Unmarshal(obsResp, &obsData); err != nil {
		s.sendToolError(id, fmt.Sprintf("Failed to parse observation data: %v", err))
		return
	}

	// Format the observation
	props := obsData.Properties
	var output string
	output += fmt.Sprintf("Current Weather Observation\n")
	output += fmt.Sprintf("Location: %.4f, %.4f\n", lat, lon)
	output += fmt.Sprintf("Station: %s (%s)\n", stationsData.Features[0].Properties.Name, stationID)
	output += fmt.Sprintf("Time: %s\n\n", props.Timestamp)
	
	if props.TextDescription != "" {
		output += fmt.Sprintf("Conditions: %s\n", props.TextDescription)
	}
	
	if props.Temperature.Value != nil {
		tempC := *props.Temperature.Value
		tempF := (tempC * 9 / 5) + 32
		output += fmt.Sprintf("Temperature: %.1f°C (%.1f°F)\n", tempC, tempF)
	}
	
	if props.Dewpoint.Value != nil {
		dewC := *props.Dewpoint.Value
		dewF := (dewC * 9 / 5) + 32
		output += fmt.Sprintf("Dewpoint: %.1f°C (%.1f°F)\n", dewC, dewF)
	}
	
	if props.RelativeHumidity.Value != nil {
		output += fmt.Sprintf("Humidity: %.0f%%\n", *props.RelativeHumidity.Value)
	}
	
	if props.WindSpeed.Value != nil && props.WindDirection.Value != nil {
		windKmh := *props.WindSpeed.Value
		windMph := windKmh * 0.621371
		output += fmt.Sprintf("Wind: %.0f° at %.1f km/h (%.1f mph)\n", *props.WindDirection.Value, windKmh, windMph)
	}
	
	if props.WindGust.Value != nil {
		gustKmh := *props.WindGust.Value
		gustMph := gustKmh * 0.621371
		output += fmt.Sprintf("Wind Gust: %.1f km/h (%.1f mph)\n", gustKmh, gustMph)
	}
	
	if props.BarometricPressure.Value != nil {
		pressurePa := *props.BarometricPressure.Value
		pressureInHg := pressurePa * 0.0002953
		output += fmt.Sprintf("Pressure: %.0f Pa (%.2f inHg)\n", pressurePa, pressureInHg)
	}
	
	if props.Visibility.Value != nil {
		visM := *props.Visibility.Value
		visMiles := visM * 0.000621371
		output += fmt.Sprintf("Visibility: %.0f m (%.1f miles)\n", visM, visMiles)
	}
	
	if props.PrecipitationLastHour.Value != nil && *props.PrecipitationLastHour.Value > 0 {
		precipMm := *props.PrecipitationLastHour.Value * 1000
		precipIn := precipMm * 0.0393701
		output += fmt.Sprintf("Precipitation (last hour): %.1f mm (%.2f in)\n", precipMm, precipIn)
	}

	result := ToolResult{
		Content: []ContentItem{
			{
				Type: "text",
				Text: output,
			},
		},
	}

	s.sendResponse(id, result)
}

func (s *MCPServer) makeRequest(url string) ([]byte, error) {
	ctx := context.Background()
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	// NOAA requires a User-Agent header
	req.Header.Set("User-Agent", "(Hunter3 MCP Weather Plugin, hunter3@example.com)")
	req.Header.Set("Accept", "application/geo+json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	return body, nil
}

func (s *MCPServer) sendToolError(id interface{}, message string) {
	logger.Printf("Tool error: %s\n", message)
	result := ToolResult{
		Content: []ContentItem{
			{
				Type: "text",
				Text: message,
			},
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
