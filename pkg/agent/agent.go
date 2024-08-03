package agent

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"cloud.google.com/go/vertexai/genai"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/minherz/wastewise/pkg/utils"
)

const (
	modelNameEnvVar = "GEMINI_MODEL_NAME"
	// from https://cloud.google.com/vertex-ai/generative-ai/docs/learn/model-versions
	defaultModelName = "gemini-1.5-flash-001"
)

var (
	systemInstructions = []string{
		"You are a friendly and helpful waste sorting assistant.",
		"When asked you help to sort waste to different types of carts.",
		"Answer according to cart types used in Washington state in the United States of America, unless the user explicitly specified another location and waste collection company.",
		"Ensure your answers are concise, unless the user requests a more complete approach.",
		"When presented with inquiries seeking information, provide answers that reflect a deep understanding of the field, guaranteeing their correctness.",
		"For prompts involving reasoning, provide a clear explanation of each step in the reasoning process before presenting the final answer.",
		"For any non-English queries, respond that you understand English only.",
	}
)

type Agent struct {
	c        *genai.Client
	m        *genai.GenerativeModel
	sessions map[string]*ChatSession
}

type ChatSession struct {
	id   string
	chat *genai.ChatSession
}

func NewAgent(ctx context.Context, e *echo.Echo) (*Agent, error) {
	var (
		projectID, region string
		err               error
	)
	if projectID, err = utils.ProjectID(ctx); err != nil {
		return nil, fmt.Errorf("could not retrieve current project ID: %w", err)
	}
	if region, err = utils.Region(ctx); err != nil {
		return nil, fmt.Errorf("could not retrieve current region: %w", err)
	}
	agent := &Agent{sessions: make(map[string]*ChatSession)}
	if agent.c, err = genai.NewClient(ctx, projectID, region); err != nil {
		return nil, fmt.Errorf("could not initialize Vertex AI client: %w", err)
	}
	modelName := utils.GetenvWithDefault(modelNameEnvVar, defaultModelName)
	agent.m = agent.c.GenerativeModel(modelName)
	agent.m.SystemInstruction = &genai.Content{
		Parts: []genai.Part{genai.Text(strings.Join(systemInstructions, " "))},
	}
	slog.Debug("initialized vertex ai", "project", projectID, "region", region, "model", modelName)

	// setup handlers
	e.POST("/ask", agent.onAsk)

	return agent, nil
}

func (a *Agent) Close() {
	if a.c != nil {
		a.Close()
	}
}

type ReturnStatus struct {
	Error   string      `json:"error,omitempty"`
	Payload interface{} `json:"payload,omitempty"`
}

type AskRequest struct {
	SessionID string `json:"sessionId,omitempty"`
	Message   string `json:"message,omitempty"`
}

type AskResponse struct {
	SessionID string `json:"sessionId,omitempty"`
	Response  string `json:"response,omitempty"`
}

func (a *Agent) onAsk(ectx echo.Context) error {
	r := &AskRequest{}
	if err := ectx.Bind(&r); err != nil {
		slog.Error("failed to parse input", "error", fmt.Sprintf("%q", err))
		return ectx.JSON(http.StatusBadRequest, ReturnStatus{Error: fmt.Sprintf("invalid input: %q", err)})
	}
	if err := checkParams(r); err != nil {
		slog.Error("invalid input", "error", fmt.Sprintf("%q", err))
		return ectx.JSON(http.StatusBadRequest, ReturnStatus{Error: fmt.Sprintf("%q", err)})
	}
	s := a.getOrCreateSession(r.SessionID)
	resp, err := s.chat.SendMessage(ectx.Request().Context(), genai.Text(r.Message))
	if err != nil {
		slog.Error("chat response error", "error", fmt.Sprintf("%q", err))
		return ectx.JSON(http.StatusInternalServerError, ReturnStatus{Error: fmt.Sprintf("chat response error: %q", err)})
	}
	if len(resp.Candidates) == 0 || resp.Candidates[0] == nil {
		return ectx.JSON(http.StatusOK, ReturnStatus{Payload: AskResponse{SessionID: r.SessionID, Response: "<empty>"}})
	}
	msg := processContent(resp.Candidates[0].Content)
	slog.Debug("ask request processed", "session", r.SessionID, "prompt", r.Message, "response", msg)
	return ectx.JSON(http.StatusOK, ReturnStatus{Payload: AskResponse{SessionID: r.SessionID, Response: msg}})
}

func checkParams(r *AskRequest) error {
	if r.Message == "" {
		return fmt.Errorf("request message is empty")
	}
	if r.SessionID == "" {
		uuid, err := uuid.NewRandom()
		if err != nil {
			return fmt.Errorf("cannot generate session ID: %w", err)
		}
		r.SessionID = uuid.String()
	}
	return nil
}

func (a *Agent) getOrCreateSession(sessionID string) *ChatSession {
	s, ok := a.sessions[sessionID]
	if !ok {
		s = &ChatSession{id: sessionID, chat: a.m.StartChat()}
		a.sessions[sessionID] = s
	}
	return s
}

func processContent(c *genai.Content) string {
	text := make([]string, len(c.Parts))
	for _, part := range c.Parts {
		if t, ok := part.(genai.Text); !ok || len(string(t)) > 0 {
			text = append(text, string(t))
		}
	}
	return strings.Join(text, ". ")
}
