package hooks

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/soyeahso/hunter3/internal/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testManager() *Manager {
	return NewManager(logging.New(nil, "silent"))
}

func TestManager_On_And_Emit(t *testing.T) {
	m := testManager()

	var called bool
	m.On(EventGatewayStart, "test", func(_ context.Context, p Payload) error {
		called = true
		assert.Equal(t, EventGatewayStart, p.Event)
		return nil
	})

	m.Emit(context.Background(), EventGatewayStart, nil)
	assert.True(t, called)
}

func TestManager_Emit_MultipleHandlers(t *testing.T) {
	m := testManager()

	var order []string
	m.On(EventMessageReceived, "first", func(_ context.Context, _ Payload) error {
		order = append(order, "first")
		return nil
	})
	m.On(EventMessageReceived, "second", func(_ context.Context, _ Payload) error {
		order = append(order, "second")
		return nil
	})

	m.Emit(context.Background(), EventMessageReceived, nil)
	assert.Equal(t, []string{"first", "second"}, order)
}

func TestManager_Emit_WithData(t *testing.T) {
	m := testManager()

	var gotData map[string]any
	m.On(EventMessageReceived, "test", func(_ context.Context, p Payload) error {
		gotData = p.Data
		return nil
	})

	m.Emit(context.Background(), EventMessageReceived, map[string]any{
		"channel": "irc",
		"from":    "alice",
	})

	assert.Equal(t, "irc", gotData["channel"])
	assert.Equal(t, "alice", gotData["from"])
}

func TestManager_Emit_HandlerError(t *testing.T) {
	m := testManager()

	var secondCalled bool
	m.On(EventGatewayStart, "failing", func(_ context.Context, _ Payload) error {
		return errors.New("handler broke")
	})
	m.On(EventGatewayStart, "second", func(_ context.Context, _ Payload) error {
		secondCalled = true
		return nil
	})

	// Should not panic; second handler should still run
	m.Emit(context.Background(), EventGatewayStart, nil)
	assert.True(t, secondCalled)
}

func TestManager_Emit_NoHandlers(t *testing.T) {
	m := testManager()
	// Should not panic
	m.Emit(context.Background(), EventGatewayStop, nil)
}

func TestManager_Off(t *testing.T) {
	m := testManager()

	var callCount int
	m.On(EventGatewayStart, "removable", func(_ context.Context, _ Payload) error {
		callCount++
		return nil
	})

	m.Emit(context.Background(), EventGatewayStart, nil)
	assert.Equal(t, 1, callCount)

	m.Off(EventGatewayStart, "removable")
	m.Emit(context.Background(), EventGatewayStart, nil)
	assert.Equal(t, 1, callCount) // should not have been called again
}

func TestManager_Off_KeepsOthers(t *testing.T) {
	m := testManager()

	var keepCalled int
	m.On(EventGatewayStart, "remove-me", func(_ context.Context, _ Payload) error { return nil })
	m.On(EventGatewayStart, "keep-me", func(_ context.Context, _ Payload) error {
		keepCalled++
		return nil
	})

	m.Off(EventGatewayStart, "remove-me")
	m.Emit(context.Background(), EventGatewayStart, nil)
	assert.Equal(t, 1, keepCalled)
}

func TestManager_EmitAsync(t *testing.T) {
	m := testManager()

	var count atomic.Int32
	var wg sync.WaitGroup
	wg.Add(2)

	m.On(EventMessageSending, "async1", func(_ context.Context, _ Payload) error {
		count.Add(1)
		wg.Done()
		return nil
	})
	m.On(EventMessageSending, "async2", func(_ context.Context, _ Payload) error {
		count.Add(1)
		wg.Done()
		return nil
	})

	m.EmitAsync(context.Background(), EventMessageSending, nil)

	// Wait with timeout
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("async handlers did not complete in time")
	}

	assert.Equal(t, int32(2), count.Load())
}

func TestManager_Count(t *testing.T) {
	m := testManager()

	assert.Equal(t, 0, m.Count(EventGatewayStart))

	m.On(EventGatewayStart, "h1", func(_ context.Context, _ Payload) error { return nil })
	assert.Equal(t, 1, m.Count(EventGatewayStart))

	m.On(EventGatewayStart, "h2", func(_ context.Context, _ Payload) error { return nil })
	assert.Equal(t, 2, m.Count(EventGatewayStart))
}

func TestManager_Events(t *testing.T) {
	m := testManager()

	m.On(EventGatewayStart, "h1", func(_ context.Context, _ Payload) error { return nil })
	m.On(EventMessageReceived, "h2", func(_ context.Context, _ Payload) error { return nil })

	events := m.Events()
	assert.Len(t, events, 2)
	assert.Contains(t, events, EventGatewayStart)
	assert.Contains(t, events, EventMessageReceived)
}

func TestAllEvents_NotEmpty(t *testing.T) {
	require.NotEmpty(t, AllEvents)
	assert.Contains(t, AllEvents, EventGatewayStart)
	assert.Contains(t, AllEvents, EventMessageReceived)
}
