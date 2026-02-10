package plugin

import (
	"context"
	"testing"

	"github.com/soyeahso/hunter3/internal/hooks"
	"github.com/soyeahso/hunter3/internal/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testPlugin struct {
	id        string
	name      string
	version   string
	initErr   error
	closeErr  error
	initCalls int
	closeCalls int
}

func (p *testPlugin) ID() string      { return p.id }
func (p *testPlugin) Name() string    { return p.name }
func (p *testPlugin) Version() string { return p.version }
func (p *testPlugin) Init(_ context.Context, _ API) error {
	p.initCalls++
	return p.initErr
}
func (p *testPlugin) Close() error {
	p.closeCalls++
	return p.closeErr
}

func testRegistry() *Registry {
	log := logging.New(nil, "silent")
	hm := hooks.NewManager(log)
	return NewRegistry(hm, log)
}

func TestRegistry_Register(t *testing.T) {
	reg := testRegistry()
	p := &testPlugin{id: "test", name: "Test Plugin", version: "1.0"}

	err := reg.Register(p)
	require.NoError(t, err)
	assert.Equal(t, 1, reg.Count())
}

func TestRegistry_Register_Duplicate(t *testing.T) {
	reg := testRegistry()
	p := &testPlugin{id: "test", name: "Test", version: "1.0"}

	require.NoError(t, reg.Register(p))
	err := reg.Register(p)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already registered")
}

func TestRegistry_Get(t *testing.T) {
	reg := testRegistry()
	p := &testPlugin{id: "test", name: "Test", version: "1.0"}
	reg.Register(p)

	got := reg.Get("test")
	assert.Equal(t, "test", got.ID())

	assert.Nil(t, reg.Get("nonexistent"))
}

func TestRegistry_List(t *testing.T) {
	reg := testRegistry()
	reg.Register(&testPlugin{id: "a", name: "A", version: "1"})
	reg.Register(&testPlugin{id: "b", name: "B", version: "1"})

	ids := reg.List()
	assert.Equal(t, []string{"a", "b"}, ids)
}

func TestRegistry_InitAll(t *testing.T) {
	reg := testRegistry()
	p1 := &testPlugin{id: "a", name: "A", version: "1"}
	p2 := &testPlugin{id: "b", name: "B", version: "1"}
	reg.Register(p1)
	reg.Register(p2)

	err := reg.InitAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, p1.initCalls)
	assert.Equal(t, 1, p2.initCalls)
}

func TestRegistry_InitAll_Error(t *testing.T) {
	reg := testRegistry()
	p := &testPlugin{id: "bad", name: "Bad", version: "1", initErr: assert.AnError}
	reg.Register(p)

	err := reg.InitAll(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bad")
}

func TestRegistry_CloseAll(t *testing.T) {
	reg := testRegistry()
	p1 := &testPlugin{id: "a", name: "A", version: "1"}
	p2 := &testPlugin{id: "b", name: "B", version: "1"}
	reg.Register(p1)
	reg.Register(p2)

	reg.CloseAll()
	assert.Equal(t, 1, p1.closeCalls)
	assert.Equal(t, 1, p2.closeCalls)
}

func TestRegistry_Info(t *testing.T) {
	reg := testRegistry()
	reg.Register(&testPlugin{id: "x", name: "Plugin X", version: "2.0"})

	infos := reg.Info()
	require.Len(t, infos, 1)
	assert.Equal(t, "x", infos[0].ID)
	assert.Equal(t, "Plugin X", infos[0].Name)
	assert.Equal(t, "2.0", infos[0].Version)
}
