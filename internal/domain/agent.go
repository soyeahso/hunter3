package domain

// Agent represents a configured AI agent.
type Agent struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Model     string `json:"model"`
	Workspace string `json:"workspace,omitempty"`
	IsDefault bool   `json:"isDefault,omitempty"`
}
