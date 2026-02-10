package main

import (
	"encoding/json"
	"testing"
)

func TestJSONRPCRequestParsing(t *testing.T) {
	testCases := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "valid initialize request",
			input:   `{"jsonrpc":"2.0","id":1,"method":"initialize"}`,
			wantErr: false,
		},
		{
			name:    "valid tools/list request",
			input:   `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
			wantErr: false,
		},
		{
			name:    "invalid json",
			input:   `{invalid}`,
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var req JSONRPCRequest
			err := json.Unmarshal([]byte(tc.input), &req)
			if (err != nil) != tc.wantErr {
				t.Errorf("Unmarshal() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestToolResultSerialization(t *testing.T) {
	result := ToolResult{
		Content: []ContentItem{
			{Type: "text", Text: "Hello, Docker!"},
		},
		IsError: false,
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Failed to marshal ToolResult: %v", err)
	}

	var decoded ToolResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal ToolResult: %v", err)
	}

	if len(decoded.Content) != 1 {
		t.Errorf("Expected 1 content item, got %d", len(decoded.Content))
	}

	if decoded.Content[0].Text != "Hello, Docker!" {
		t.Errorf("Expected 'Hello, Docker!', got '%s'", decoded.Content[0].Text)
	}
}

func TestGetString(t *testing.T) {
	args := map[string]interface{}{
		"image":   "nginx:latest",
		"number":  42,
		"missing": nil,
	}

	tests := []struct {
		key      string
		expected string
	}{
		{"image", "nginx:latest"},
		{"number", ""},
		{"missing", ""},
		{"notfound", ""},
	}

	for _, tt := range tests {
		result := getString(args, tt.key)
		if result != tt.expected {
			t.Errorf("getString(%q) = %q, want %q", tt.key, result, tt.expected)
		}
	}
}

func TestGetBool(t *testing.T) {
	args := map[string]interface{}{
		"detach":  true,
		"remove":  false,
		"string":  "true",
		"missing": nil,
	}

	tests := []struct {
		key      string
		expected bool
	}{
		{"detach", true},
		{"remove", false},
		{"string", false},
		{"missing", false},
		{"notfound", false},
	}

	for _, tt := range tests {
		result := getBool(args, tt.key)
		if result != tt.expected {
			t.Errorf("getBool(%q) = %v, want %v", tt.key, result, tt.expected)
		}
	}
}

func TestGetStringArray(t *testing.T) {
	args := map[string]interface{}{
		"ports": []interface{}{"8080:80", "443:443"},
		"empty": []interface{}{},
		"mixed": []interface{}{"string", 42, true},
		"nil":   nil,
	}

	tests := []struct {
		key      string
		expected []string
	}{
		{"ports", []string{"8080:80", "443:443"}},
		{"empty", []string{}},
		{"mixed", []string{"string"}},
		{"nil", nil},
		{"notfound", nil},
	}

	for _, tt := range tests {
		result := getStringArray(args, tt.key)
		if len(result) != len(tt.expected) {
			t.Errorf("getStringArray(%q) length = %d, want %d", tt.key, len(result), len(tt.expected))
			continue
		}
		for i := range result {
			if result[i] != tt.expected[i] {
				t.Errorf("getStringArray(%q)[%d] = %q, want %q", tt.key, i, result[i], tt.expected[i])
			}
		}
	}
}

func TestDockerResultSerialization(t *testing.T) {
	result := DockerResult{
		Command: "docker ps -a",
		Success: true,
		Stdout:  "CONTAINER ID   IMAGE     COMMAND",
		Stderr:  "",
		Error:   "",
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Failed to marshal DockerResult: %v", err)
	}

	var decoded DockerResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal DockerResult: %v", err)
	}

	if decoded.Command != result.Command {
		t.Errorf("Command mismatch: got %q, want %q", decoded.Command, result.Command)
	}

	if decoded.Success != result.Success {
		t.Errorf("Success mismatch: got %v, want %v", decoded.Success, result.Success)
	}
}

func TestPropertyConstructors(t *testing.T) {
	// Test stringProp
	prop := stringProp("Test description")
	if prop.Type != "string" || prop.Description != "Test description" {
		t.Errorf("stringProp failed: got %+v", prop)
	}

	// Test stringPropDefault
	propDefault := stringPropDefault("Test with default", "default_value")
	if propDefault.Default != "default_value" {
		t.Errorf("stringPropDefault failed: got %+v", propDefault)
	}

	// Test stringArrayProp
	arrayProp := stringArrayProp("Array description")
	if arrayProp.Type != "array" || arrayProp.Items == nil || arrayProp.Items.Type != "string" {
		t.Errorf("stringArrayProp failed: got %+v", arrayProp)
	}

	// Test boolProp
	boolProperty := boolProp("Boolean description")
	if boolProperty.Type != "boolean" {
		t.Errorf("boolProp failed: got %+v", boolProperty)
	}
}
