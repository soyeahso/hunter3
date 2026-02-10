package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- ParseConfigPath extended tests ---

func TestParseConfigPath_Extended(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []string
		wantErr bool
	}{
		{"single segment", "gateway", []string{"gateway"}, false},
		{"two segments", "gateway.port", []string{"gateway", "port"}, false},
		{"three segments", "gateway.auth.mode", []string{"gateway", "auth", "mode"}, false},
		{"empty", "", nil, true},
		{"empty segment", "gateway..port", nil, true},
		{"leading dot", ".gateway", nil, true},
		{"trailing dot", "gateway.", nil, true},
		{"blocked __proto__", "foo.__proto__.bar", nil, true},
		{"blocked prototype", "prototype.x", nil, true},
		{"blocked constructor", "constructor", nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseConfigPath(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				var ce *ConfigError
				assert.ErrorAs(t, err, &ce)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

// --- GetValueAtPath extended tests ---

func TestGetValueAtPath_Extended(t *testing.T) {
	root := map[string]any{
		"gateway": map[string]any{
			"port": 18789,
			"auth": map[string]any{
				"mode": "token",
			},
		},
		"simple": "value",
	}

	tests := []struct {
		name string
		path []string
		want any
		ok   bool
	}{
		{"nested value", []string{"gateway", "port"}, 18789, true},
		{"deeply nested", []string{"gateway", "auth", "mode"}, "token", true},
		{"top level", []string{"simple"}, "value", true},
		{"missing key", []string{"nonexistent"}, nil, false},
		{"missing nested", []string{"gateway", "nonexistent"}, nil, false},
		{"non-map intermediate", []string{"simple", "sub"}, nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, ok := GetValueAtPath(root, tt.path)
			assert.Equal(t, tt.ok, ok)
			if tt.ok {
				assert.Equal(t, tt.want, val)
			}
		})
	}
}

// --- SetValueAtPath extended tests ---

func TestSetValueAtPath_Update(t *testing.T) {
	root := map[string]any{
		"gateway": map[string]any{
			"port": 18789,
		},
	}

	SetValueAtPath(root, []string{"gateway", "port"}, 9999)
	val, ok := GetValueAtPath(root, []string{"gateway", "port"})
	assert.True(t, ok)
	assert.Equal(t, 9999, val)
}

func TestSetValueAtPath_CreatesIntermediates(t *testing.T) {
	root := map[string]any{}

	SetValueAtPath(root, []string{"a", "b", "c"}, "deep")
	val, ok := GetValueAtPath(root, []string{"a", "b", "c"})
	assert.True(t, ok)
	assert.Equal(t, "deep", val)
}

func TestSetValueAtPath_OverwritesNonMap(t *testing.T) {
	root := map[string]any{
		"gateway": "string-not-map",
	}

	SetValueAtPath(root, []string{"gateway", "port"}, 8080)
	val, ok := GetValueAtPath(root, []string{"gateway", "port"})
	assert.True(t, ok)
	assert.Equal(t, 8080, val)
}

func TestSetValueAtPath_SingleKey(t *testing.T) {
	root := map[string]any{}

	SetValueAtPath(root, []string{"version"}, "1.0.0")
	assert.Equal(t, "1.0.0", root["version"])
}

// --- UnsetValueAtPath extended tests ---

func TestUnsetValueAtPath_PreserveSiblings(t *testing.T) {
	root := map[string]any{
		"gateway": map[string]any{
			"port": 18789,
			"mode": "local",
		},
	}

	ok := UnsetValueAtPath(root, []string{"gateway", "port"})
	assert.True(t, ok)

	_, found := GetValueAtPath(root, []string{"gateway", "port"})
	assert.False(t, found)

	val, found := GetValueAtPath(root, []string{"gateway", "mode"})
	assert.True(t, found)
	assert.Equal(t, "local", val)
}

func TestUnsetValueAtPath_NotFound(t *testing.T) {
	root := map[string]any{
		"gateway": map[string]any{
			"port": 18789,
		},
	}

	ok := UnsetValueAtPath(root, []string{"gateway", "nonexistent"})
	assert.False(t, ok)
}

func TestUnsetValueAtPath_MissingIntermediate(t *testing.T) {
	root := map[string]any{}
	ok := UnsetValueAtPath(root, []string{"a", "b", "c"})
	assert.False(t, ok)
}

func TestUnsetValueAtPath_NonMapIntermediate(t *testing.T) {
	root := map[string]any{
		"gateway": "string",
	}
	ok := UnsetValueAtPath(root, []string{"gateway", "port"})
	assert.False(t, ok)
}

// --- ResolvePaths extended tests ---

func TestResolvePaths_AllFields(t *testing.T) {
	t.Setenv("HUNTER3_HOME", "")

	paths, err := ResolvePaths()
	require.NoError(t, err)

	home, _ := os.UserHomeDir()
	assert.Equal(t, filepath.Join(home, ".hunter3"), paths.Base)
	assert.Equal(t, filepath.Join(home, ".hunter3", "config.yaml"), paths.Config)
	assert.Equal(t, filepath.Join(home, ".hunter3", "credentials"), paths.Credentials)
	assert.Equal(t, filepath.Join(home, ".hunter3", "sessions"), paths.Sessions)
	assert.Equal(t, filepath.Join(home, ".hunter3", "agents"), paths.Agents)
	assert.Equal(t, filepath.Join(home, ".hunter3", "logs"), paths.Logs)
	assert.Equal(t, filepath.Join(home, ".hunter3", "plugins"), paths.Plugins)
	assert.Equal(t, filepath.Join(home, ".hunter3", "data"), paths.Data)
}

func TestResolvePaths_CustomHomeAllFields(t *testing.T) {
	t.Setenv("HUNTER3_HOME", "/tmp/testlb")

	paths, err := ResolvePaths()
	require.NoError(t, err)

	assert.Equal(t, "/tmp/testlb", paths.Base)
	assert.Equal(t, "/tmp/testlb/config.yaml", paths.Config)
	assert.Equal(t, "/tmp/testlb/credentials", paths.Credentials)
	assert.Equal(t, "/tmp/testlb/sessions", paths.Sessions)
	assert.Equal(t, "/tmp/testlb/agents", paths.Agents)
	assert.Equal(t, "/tmp/testlb/logs", paths.Logs)
	assert.Equal(t, "/tmp/testlb/plugins", paths.Plugins)
	assert.Equal(t, "/tmp/testlb/data", paths.Data)
}

func TestEnsureDirs_CreatesAll(t *testing.T) {
	tmpDir := t.TempDir()
	paths := Paths{
		Base:        tmpDir,
		Credentials: filepath.Join(tmpDir, "credentials"),
		Sessions:    filepath.Join(tmpDir, "sessions"),
		Agents:      filepath.Join(tmpDir, "agents"),
		Logs:        filepath.Join(tmpDir, "logs"),
		Plugins:     filepath.Join(tmpDir, "plugins"),
		Data:        filepath.Join(tmpDir, "data"),
	}

	err := paths.EnsureDirs()
	require.NoError(t, err)

	for _, dir := range []string{
		paths.Base, paths.Credentials, paths.Sessions,
		paths.Agents, paths.Logs, paths.Plugins, paths.Data,
	} {
		info, err := os.Stat(dir)
		require.NoError(t, err)
		assert.True(t, info.IsDir())
	}
}

func TestEnsureDirs_Idempotent(t *testing.T) {
	tmpDir := t.TempDir()
	paths := Paths{
		Base:        tmpDir,
		Credentials: filepath.Join(tmpDir, "credentials"),
		Sessions:    filepath.Join(tmpDir, "sessions"),
		Agents:      filepath.Join(tmpDir, "agents"),
		Logs:        filepath.Join(tmpDir, "logs"),
		Plugins:     filepath.Join(tmpDir, "plugins"),
		Data:        filepath.Join(tmpDir, "data"),
	}

	require.NoError(t, paths.EnsureDirs())
	require.NoError(t, paths.EnsureDirs()) // second call should succeed
}

// --- blockedKeys tests ---

func TestBlockedKeys(t *testing.T) {
	assert.True(t, blockedKeys["__proto__"])
	assert.True(t, blockedKeys["prototype"])
	assert.True(t, blockedKeys["constructor"])
	assert.False(t, blockedKeys["gateway"])
	assert.False(t, blockedKeys["port"])
}
