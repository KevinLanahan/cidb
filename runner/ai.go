package runner

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

func analyzeFailure(command string, output string, exitCode int) string {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		return ""
	}

	client := anthropic.NewClient(option.WithAPIKey(apiKey))

	prompt := fmt.Sprintf(`A CI pipeline step failed. Explain in 2-3 sentences why it failed and how to fix it. Be specific and practical — no fluff.

Command:
%s

Output:
%s

Exit code: %d`, command, output, exitCode)

	msg, err := client.Messages.New(context.Background(), anthropic.MessageNewParams{
		Model:     "claude-haiku-4-5",
		MaxTokens: 300,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(prompt)),
		},
	})
	if err != nil {
		return ""
	}

	var sb strings.Builder
	for _, block := range msg.Content {
		if block.Type == "text" {
			sb.WriteString(block.Text)
		}
	}
	return strings.TrimSpace(sb.String())
}
