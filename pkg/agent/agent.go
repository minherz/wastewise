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
		"You are a friendly and helpful assistant for waste sorting.",
		"When asked in English you help to sort waste according to categories accepted in the United States of America.",
		"Ensure your answers are concise, unless the user requests a more complete approach.",
		"When presented with inquiries seeking information, provide answers that reflect a deep understanding of the field, guaranteeing their correctness.",
		"For any non-English queries, respond if you know waste sorting rules for the country of the language or explain that you do not have the waste sorting information otherwise in the same language as the prompt.",
		"For prompts involving reasoning, provide a clear explanation of each step in the reasoning process before presenting the final answer.",
	}
)

type Agent struct {
	VertexClient *genai.Client
	Model        *genai.GenerativeModel
	Sessions     map[string]*ChatSession
}

type ChatSession struct {
	ID      string
	Session *genai.ChatSession
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
	agent := &Agent{Sessions: make(map[string]*ChatSession)}
	if agent.VertexClient, err = genai.NewClient(ctx, projectID, region); err != nil {
		return nil, fmt.Errorf("could not initialize Vertex AI client: %w", err)
	}
	modelName := utils.GetenvWithDefault(modelNameEnvVar, defaultModelName)
	agent.Model = agent.VertexClient.GenerativeModel(modelName)
	agent.Model.SystemInstruction = &genai.Content{
		Parts: []genai.Part{genai.Text(strings.Join(systemInstructions, " "))},
	}
	slog.Debug("initialized vertex ai", "project", projectID, "region", region, "model", modelName)

	// setup handlers
	e.POST("/ask", agent.onAsk)

	return agent, nil
}

func (a *Agent) Close() {
	if a.VertexClient != nil {
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
	input := &AskRequest{}
	if err := ectx.Bind(&input); err != nil {
		slog.Error("failed to parse input", "error", fmt.Sprintf("%v", err))
		return ectx.JSON(http.StatusBadRequest, ReturnStatus{Error: fmt.Sprintf("invalid input: %q", err)})
	}
	if input.Message == "" {
		slog.Error("prompt is empty")
		return ectx.JSON(http.StatusBadRequest, ReturnStatus{Error: "prompt is empty"})
	}
	prompt := input.Message
	// TODO: augment message with additional bordering conditions
	if input.SessionID == "" {
		if ID, err := uuid.NewRandom(); err != nil {
			slog.Error("failed to generate session ID", "error", fmt.Sprintf("%v", err))
			return ectx.JSON(http.StatusInternalServerError, ReturnStatus{Error: fmt.Sprintf("failed to generate session ID: %q", err)})
		} else {
			input.SessionID = ID.String()
		}
		session := &ChatSession{ID: input.SessionID, Session: a.Model.StartChat()}
		// TODO: check for already existing session
		a.Sessions[input.SessionID] = session
	}
	s := a.Sessions[input.SessionID]
	result, err := s.Session.SendMessage(ectx.Request().Context(), genai.Text(prompt))
	if err != nil {
		slog.Error("chat response error", "error", fmt.Sprintf("%v", err))
		return ectx.JSON(http.StatusInternalServerError, ReturnStatus{Error: fmt.Sprintf("chat response error: %q", err)})
	}
	if len(result.Candidates) == 0 {
		return ectx.JSON(http.StatusOK, ReturnStatus{Payload: AskResponse{SessionID: input.SessionID, Response: "<empty>"}})
	}
	response := composeResponse(result.Candidates[0])
	slog.Debug("ask request processed", "session", input.SessionID, "prompt", prompt, "response", response)
	return ectx.JSON(http.StatusOK, ReturnStatus{Payload: AskResponse{SessionID: input.SessionID, Response: response}})
}

func composeResponse(candidate *genai.Candidate) string {
	totalParts := len(candidate.Content.Parts)
	if totalParts == 0 {
		return "<empty>"
	}
	// convert []genai.Part to []string through []genai.Text
	// TODO: skip genai.Part that aren't genai.Text
	texts := make([]string, totalParts)
	for i := range candidate.Content.Parts {
		texts[i] = string(candidate.Content.Parts[i].(genai.Text))
	}
	// concatenate as strings
	return strings.Join(texts, ". ")
}
