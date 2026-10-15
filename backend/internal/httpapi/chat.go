package httpapi

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"bugun/internal/analyst"
)

// The chat assistant (CHATBOT_REJA.md): a conversation over the analyst's
// functions, and the admins' view of what it could not answer.

func (s *Server) registerChat(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "chat", Method: http.MethodPost, Path: "/v1/chat",
		Summary: "One message to the assistant: prices, products, regions, forecasts, advice, basket and recipe costs",
		Description: "Send session_id from the previous reply to continue the conversation (follow-ups such as \"Andijonda-chi?\"). " +
			"kind is answer, clarify (a question back with options as buttons: send the chosen option as the next message) or unanswered (the admins were told). " +
			"actions are proposals to confirm with a button through their own endpoints. Every number comes from the platform's data. Signing in is optional.",
		Tags: []string{"ai"},
	}, s.chat)
	huma.Register(api, huma.Operation{
		OperationID: "adminChatUnanswered", Method: http.MethodGet, Path: "/v1/admin/chat/unanswered",
		Summary: "Questions the assistant could not answer, grouped, most asked first", Tags: []string{"admin"}, Security: authRequired,
	}, s.adminChatUnanswered)
	huma.Register(api, huma.Operation{
		OperationID: "adminChatStats", Method: http.MethodGet, Path: "/v1/admin/chat/stats",
		Summary: "The assistant's figures: answered, clarified, unanswered, model or rules, latency, functions used", Tags: []string{"admin"}, Security: authRequired,
	}, s.adminChatStats)
}

type ChatInput struct {
	SessionInput
	Body struct {
		SessionID string `json:"session_id,omitempty" maxLength:"36" doc:"From the previous reply; empty starts a conversation"`
		Message   string `json:"message" minLength:"1" maxLength:"500" doc:"e.g. \"Osh qilmoqchiman, 10 kishilik. Masalliqlar qancha turadi?\""`
		Lang      string `json:"lang,omitempty" enum:"uz,ru,en" default:"uz"`
	}
}

type ChatOutput struct {
	Body analyst.ChatAnswer
}

func (s *Server) chat(ctx context.Context, in *ChatInput) (*ChatOutput, error) {
	an := s.aiDeps().Analyst
	if an == nil {
		return nil, featureOff()
	}
	key, u := s.visitor(ctx, in.Session)
	a, err := an.Converse(ctx, analyst.ChatRequest{SessionID: in.Body.SessionID, Message: in.Body.Message, Lang: in.Body.Lang, User: u, Key: key})
	if errors.Is(err, analyst.ErrEmpty) {
		return nil, apiError(http.StatusUnprocessableEntity, "question_invalid", "Savol boʻsh yoki juda uzun.")
	}
	if err != nil {
		return nil, internalError(ctx, "chat", err)
	}
	return &ChatOutput{Body: a}, nil
}

type ChatDaysInput struct {
	SessionInput
	Days int `query:"days" minimum:"1" maximum:"90" default:"7"`
}

type ChatUnansweredOutput struct {
	Body struct {
		Questions []analyst.UnansweredGroup `json:"questions"`
	}
}

func (s *Server) adminChatUnanswered(ctx context.Context, in *ChatDaysInput) (*ChatUnansweredOutput, error) {
	if _, err := s.currentAdmin(ctx, in.Session); err != nil {
		return nil, err
	}
	an := s.aiDeps().Analyst
	if an == nil {
		return nil, featureOff()
	}
	list, err := an.Unanswered(ctx, in.Days)
	if err != nil {
		return nil, internalError(ctx, "chat unanswered", err)
	}
	out := &ChatUnansweredOutput{}
	out.Body.Questions = list
	return out, nil
}

type ChatStatsOutput struct {
	Body analyst.ChatStats
}

func (s *Server) adminChatStats(ctx context.Context, in *ChatDaysInput) (*ChatStatsOutput, error) {
	if _, err := s.currentAdmin(ctx, in.Session); err != nil {
		return nil, err
	}
	an := s.aiDeps().Analyst
	if an == nil {
		return nil, featureOff()
	}
	st, err := an.Stats(ctx, in.Days)
	if err != nil {
		return nil, internalError(ctx, "chat stats", err)
	}
	return &ChatStatsOutput{Body: st}, nil
}
