package agent

import (
	"fmt"
	"strings"
	"time"
)

// PromptConfig controls system prompt generation.
type PromptConfig struct {
	AgentName   string
	AgentID     string
	Model       string
	Tools       []ToolDef
	ChannelID   string
	ChatType    string
	UserName    string
	ExtraPrompt string
}

// BuildSystemPrompt constructs the system prompt for the LLM.
func BuildSystemPrompt(cfg PromptConfig) string {
	var b strings.Builder

	// Identity
	//b.WriteString(fmt.Sprintf("You are %s, a helpful AI assistant powered by Hunter3.\n\n", cfg.AgentName))

	// Date context
	b.WriteString(fmt.Sprintf("Current date: %s\n", time.Now().Format("2006-01-02")))

	// Channel context
	if cfg.ChannelID != "" {
		b.WriteString(fmt.Sprintf("Channel: %s\n", cfg.ChannelID))
	}
	if cfg.ChatType != "" {
		b.WriteString(fmt.Sprintf("Chat type: %s\n", cfg.ChatType))
	}
	if cfg.UserName != "" {
		b.WriteString(fmt.Sprintf("User: %s\n", cfg.UserName))
	}

	b.WriteString("\n")

	// Guidelines
	b.WriteString("Guidelines:\n")
	//b.WriteString("- Be helpful, concise, and accurate.\n")
	//b.WriteString("- If you don't know something, say so rather than guessing.\n")
	b.WriteString("- When using tools, explain what you're doing.\n")
	//b.WriteString("- Format responses with markdown when helpful.\n")

	// Tool definitions
	if len(cfg.Tools) > 0 {
		b.WriteString("\n## Available Tools\n\n")
		b.WriteString("You can call tools by outputting a fenced code block with the language tag `tool_call`:\n\n")
		b.WriteString("```tool_call\n{\"tool\": \"tool_name\", \"input\": {\"param\": \"value\"}}\n```\n\n")
		b.WriteString("After a tool is executed, the result will be provided. You may call multiple tools before giving your final response.\n\n")
		for _, t := range cfg.Tools {
			fmt.Fprintf(&b, "### %s\n%s\n", t.Name, t.Description)
			if t.InputSchema != "" {
				fmt.Fprintf(&b, "Input schema: %s\n", t.InputSchema)
			}
			b.WriteString("\n")
		}
	}

	// Extra/custom prompt
	if cfg.ExtraPrompt != "" {
		b.WriteString("\n")
		b.WriteString(cfg.ExtraPrompt)
		b.WriteString("\n")
	}

	return b.String()
}
