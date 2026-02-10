package llm

import "context"

// MockClient is a test double for Client.
type MockClient struct {
	ProviderName  string
	CompleteFunc  func(ctx context.Context, req CompletionRequest) (*CompletionResponse, error)
	StreamFunc    func(ctx context.Context, req CompletionRequest) (<-chan StreamEvent, error)
}

func (m *MockClient) Name() string { return m.ProviderName }

func (m *MockClient) Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	if m.CompleteFunc != nil {
		return m.CompleteFunc(ctx, req)
	}
	return &CompletionResponse{Content: "mock response"}, nil
}

func (m *MockClient) Stream(ctx context.Context, req CompletionRequest) (<-chan StreamEvent, error) {
	if m.StreamFunc != nil {
		return m.StreamFunc(ctx, req)
	}
	ch := make(chan StreamEvent, 2)
	ch <- StreamEvent{Type: "delta", Content: "mock "}
	ch <- StreamEvent{
		Type: "done",
		Response: &CompletionResponse{Content: "mock stream response"},
	}
	close(ch)
	return ch, nil
}
