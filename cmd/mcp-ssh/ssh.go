package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

// SSHSession represents an active SSH connection
type SSHSession struct {
	Name      string
	Host      string
	Port      int
	Username  string
	Client    *ssh.Client
	Connected time.Time
}

// SSHManager manages SSH connections
type SSHManager struct {
	sessions map[string]*SSHSession
	mu       sync.RWMutex
}

// NewSSHManager creates a new SSH manager
func NewSSHManager() *SSHManager {
	return &SSHManager{
		sessions: make(map[string]*SSHSession),
	}
}

// Helper function to get string parameter
func getStringParam(args map[string]interface{}, key string) string {
	if val, ok := args[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

// Helper function to get number parameter
func getNumberParam(args map[string]interface{}, key string, defaultVal float64) float64 {
	if val, ok := args[key]; ok {
		switch v := val.(type) {
		case float64:
			return v
		case int:
			return float64(v)
		case int64:
			return float64(v)
		}
	}
	return defaultVal
}

func errorResult(msg string) ToolResult {
	return ToolResult{
		Content: []ContentItem{{Type: "text", Text: msg}},
		IsError: true,
	}
}

func successResult(msg string) ToolResult {
	return ToolResult{
		Content: []ContentItem{{Type: "text", Text: msg}},
	}
}

// handleConnect handles SSH connection requests
func (m *SSHManager) handleConnect(args map[string]interface{}) ToolResult {
	host := getStringParam(args, "host")
	if host == "" {
		return errorResult("Error: host is required")
	}

	username := getStringParam(args, "username")
	if username == "" {
		return errorResult("Error: username is required")
	}

	port := int(getNumberParam(args, "port", 22))
	password := getStringParam(args, "password")
	keyPath := getStringParam(args, "key_path")
	keyPassphrase := getStringParam(args, "key_passphrase")
	sessionName := getStringParam(args, "session_name")

	if password == "" && keyPath == "" {
		return errorResult("Error: either password or key_path must be provided")
	}

	if sessionName == "" {
		sessionName = fmt.Sprintf("%s@%s", username, host)
	}

	m.mu.RLock()
	if _, exists := m.sessions[sessionName]; exists {
		m.mu.RUnlock()
		return errorResult(fmt.Sprintf("Error: session '%s' already exists", sessionName))
	}
	m.mu.RUnlock()

	config := &ssh.ClientConfig{
		User:            username,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         30 * time.Second,
	}

	if password != "" {
		config.Auth = append(config.Auth, ssh.Password(password))
	}

	if keyPath != "" {
		key, err := os.ReadFile(keyPath)
		if err != nil {
			return errorResult(fmt.Sprintf("Error: failed to read private key: %v", err))
		}

		var signer ssh.Signer
		if keyPassphrase != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase(key, []byte(keyPassphrase))
		} else {
			signer, err = ssh.ParsePrivateKey(key)
		}
		if err != nil {
			return errorResult(fmt.Sprintf("Error: failed to parse private key: %v", err))
		}

		config.Auth = append(config.Auth, ssh.PublicKeys(signer))
	}

	addr := fmt.Sprintf("%s:%d", host, port)
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return errorResult(fmt.Sprintf("Error: failed to connect: %v", err))
	}

	session := &SSHSession{
		Name:      sessionName,
		Host:      host,
		Port:      port,
		Username:  username,
		Client:    client,
		Connected: time.Now(),
	}

	m.mu.Lock()
	m.sessions[sessionName] = session
	m.mu.Unlock()

	return successResult(fmt.Sprintf("Successfully connected to %s@%s:%d\nSession name: %s", username, host, port, sessionName))
}

// handleExecute handles SSH command execution
func (m *SSHManager) handleExecute(args map[string]interface{}) ToolResult {
	sessionName := getStringParam(args, "session_name")
	command := getStringParam(args, "command")
	workingDir := getStringParam(args, "working_dir")
	timeout := int(getNumberParam(args, "timeout", 300))

	if sessionName == "" {
		return errorResult("Error: session_name is required")
	}
	if command == "" {
		return errorResult("Error: command is required")
	}

	m.mu.RLock()
	session, exists := m.sessions[sessionName]
	m.mu.RUnlock()

	if !exists {
		return errorResult(fmt.Sprintf("Error: session '%s' not found", sessionName))
	}

	sshSession, err := session.Client.NewSession()
	if err != nil {
		return errorResult(fmt.Sprintf("Error: failed to create session: %v", err))
	}
	defer sshSession.Close()

	var stdout, stderr bytes.Buffer
	sshSession.Stdout = &stdout
	sshSession.Stderr = &stderr

	if workingDir != "" {
		command = fmt.Sprintf("cd %s && %s", workingDir, command)
	}

	done := make(chan error, 1)
	go func() {
		done <- sshSession.Run(command)
	}()

	select {
	case <-time.After(time.Duration(timeout) * time.Second):
		sshSession.Signal(ssh.SIGKILL)
		return errorResult(fmt.Sprintf("Error: command timed out after %d seconds", timeout))
	case err := <-done:
		result := fmt.Sprintf("Command: %s\n\n", command)

		if stdout.Len() > 0 {
			result += fmt.Sprintf("STDOUT:\n%s\n", stdout.String())
		}

		if stderr.Len() > 0 {
			result += fmt.Sprintf("STDERR:\n%s\n", stderr.String())
		}

		if err != nil {
			result += fmt.Sprintf("\nExit Status: %v", err)
		} else {
			result += "\nExit Status: 0 (success)"
		}

		return successResult(result)
	}
}

// handleUpload handles file upload via SCP
func (m *SSHManager) handleUpload(args map[string]interface{}) ToolResult {
	sessionName := getStringParam(args, "session_name")
	localPath := getStringParam(args, "local_path")
	remotePath := getStringParam(args, "remote_path")
	permissions := getStringParam(args, "permissions")

	if sessionName == "" || localPath == "" || remotePath == "" {
		return errorResult("Error: session_name, local_path, and remote_path are required")
	}

	m.mu.RLock()
	session, exists := m.sessions[sessionName]
	m.mu.RUnlock()

	if !exists {
		return errorResult(fmt.Sprintf("Error: session '%s' not found", sessionName))
	}

	data, err := os.ReadFile(localPath)
	if err != nil {
		return errorResult(fmt.Sprintf("Error: failed to read local file: %v", err))
	}

	remoteDir := filepath.Dir(remotePath)
	if err := m.execCommand(session, fmt.Sprintf("mkdir -p %s", remoteDir)); err != nil {
		return errorResult(fmt.Sprintf("Error: failed to create remote directory: %v", err))
	}

	sshSession, err := session.Client.NewSession()
	if err != nil {
		return errorResult(fmt.Sprintf("Error: failed to create session: %v", err))
	}
	defer sshSession.Close()

	perm := "0644"
	if permissions != "" {
		perm = permissions
	}

	w, err := sshSession.StdinPipe()
	if err != nil {
		return errorResult(fmt.Sprintf("Error: failed to create stdin pipe: %v", err))
	}

	if err := sshSession.Start(fmt.Sprintf("scp -t %s", remotePath)); err != nil {
		return errorResult(fmt.Sprintf("Error: failed to start scp: %v", err))
	}

	fmt.Fprintf(w, "C%s %d %s\n", perm, len(data), filepath.Base(remotePath))
	w.Write(data)
	fmt.Fprint(w, "\x00")
	w.Close()

	if err := sshSession.Wait(); err != nil {
		return errorResult(fmt.Sprintf("Error: failed to upload file: %v", err))
	}

	return successResult(fmt.Sprintf("Successfully uploaded %s to %s", localPath, remotePath))
}

// handleDownload handles file download via SSH
func (m *SSHManager) handleDownload(args map[string]interface{}) ToolResult {
	sessionName := getStringParam(args, "session_name")
	remotePath := getStringParam(args, "remote_path")
	localPath := getStringParam(args, "local_path")

	if sessionName == "" || remotePath == "" || localPath == "" {
		return errorResult("Error: session_name, remote_path, and local_path are required")
	}

	m.mu.RLock()
	session, exists := m.sessions[sessionName]
	m.mu.RUnlock()

	if !exists {
		return errorResult(fmt.Sprintf("Error: session '%s' not found", sessionName))
	}

	sshSession, err := session.Client.NewSession()
	if err != nil {
		return errorResult(fmt.Sprintf("Error: failed to create session: %v", err))
	}
	defer sshSession.Close()

	var stdout bytes.Buffer
	sshSession.Stdout = &stdout

	if err := sshSession.Run(fmt.Sprintf("cat %s", remotePath)); err != nil {
		return errorResult(fmt.Sprintf("Error: failed to download file: %v", err))
	}

	localDir := filepath.Dir(localPath)
	if err := os.MkdirAll(localDir, 0755); err != nil {
		return errorResult(fmt.Sprintf("Error: failed to create local directory: %v", err))
	}

	if err := os.WriteFile(localPath, stdout.Bytes(), 0644); err != nil {
		return errorResult(fmt.Sprintf("Error: failed to write local file: %v", err))
	}

	return successResult(fmt.Sprintf("Successfully downloaded %s to %s (%d bytes)", remotePath, localPath, stdout.Len()))
}

// handleListSessions lists all active SSH sessions
func (m *SSHManager) handleListSessions(args map[string]interface{}) ToolResult {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.sessions) == 0 {
		return successResult("No active SSH sessions")
	}

	var result strings.Builder
	result.WriteString("Active SSH Sessions:\n\n")

	for name, session := range m.sessions {
		result.WriteString(fmt.Sprintf("Session: %s\n", name))
		result.WriteString(fmt.Sprintf("  Host: %s:%d\n", session.Host, session.Port))
		result.WriteString(fmt.Sprintf("  Username: %s\n", session.Username))
		result.WriteString(fmt.Sprintf("  Connected: %s\n", session.Connected.Format(time.RFC3339)))
		result.WriteString("\n")
	}

	return successResult(result.String())
}

// handleDisconnect disconnects an SSH session
func (m *SSHManager) handleDisconnect(args map[string]interface{}) ToolResult {
	sessionName := getStringParam(args, "session_name")
	if sessionName == "" {
		return errorResult("Error: session_name is required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	session, exists := m.sessions[sessionName]
	if !exists {
		return errorResult(fmt.Sprintf("Error: session '%s' not found", sessionName))
	}

	if err := session.Client.Close(); err != nil {
		return errorResult(fmt.Sprintf("Error: error closing connection: %v", err))
	}

	delete(m.sessions, sessionName)

	return successResult(fmt.Sprintf("Successfully disconnected session '%s'", sessionName))
}

// execCommand is a helper to execute a simple command
func (m *SSHManager) execCommand(session *SSHSession, command string) error {
	sshSession, err := session.Client.NewSession()
	if err != nil {
		return err
	}
	defer sshSession.Close()

	return sshSession.Run(command)
}
