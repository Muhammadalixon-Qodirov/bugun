package analyst

import (
	"context"
	"fmt"
	"time"

	"bugun/internal/insight"
	"bugun/internal/llm"
)

// FirstTool asks a model which function it would call first for a question,
// with the analyst's own prompt and functions; "" when it calls none. The
// evaluation (cmd/eval) compares it with the expected function.
func FirstTool(ctx context.Context, model llm.Chatter, question, lang string) (string, error) {
	lang = langOf(lang)
	msg, err := model.Chat(llm.WithFeature(ctx, "analyst_eval"), llm.ChatRequest{
		Messages: []llm.Message{
			{Role: "system", Content: fmt.Sprintf(systemPrompt, langNames[lang], insight.Day(time.Now()).Time.Format(time.DateOnly))},
			{Role: "user", Content: question},
		},
		Tools: toolDefs(), ToolChoice: "auto", Temperature: 0, MaxTokens: 300,
	})
	if err != nil || len(msg.ToolCalls) == 0 {
		return "", err
	}
	return msg.ToolCalls[0].Function.Name, nil
}
