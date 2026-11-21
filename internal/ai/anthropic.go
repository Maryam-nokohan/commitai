package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const anthropicEndpoint = "https://api.anthropic.com/v1/messages"
const anthropicVersion = "2023-06-01"
const defaultAnthropicModel = "claude-sonnet-4-6"

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	System    string             `json:"system"`
	Messages  []anthropicMessage `json:"messages"`
}

type anthropicResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func generateAnthropic(ctx context.Context, req Request) (Response, error) {
	apiKey := strings.TrimSpace(req.APIKey)
	if apiKey == "" {
		return Response{}, errors.New("no Anthropic API key set: export ANTHROPIC_API_KEY or put it in ~/.config/commitai/.env")
	}

	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = defaultAnthropicModel
	}

	payload := anthropicRequest{
		Model:     model,
		MaxTokens: 1024,
		System:    systemPrompt(),
		Messages: []anthropicMessage{
			{Role: "user", Content: buildPrompt(req.Diff, req.Suggestions, req.Rules, req.Examples, req.BadExamples, req.GitHistory)},
		},
	}

	headers := map[string]string{
		"x-api-key":         apiKey,
		"anthropic-version": anthropicVersion,
	}
	status, body, err := postJSON(ctx, anthropicEndpoint, headers, payload, httpTimeout)
	if err != nil {
		return Response{}, fmt.Errorf("API request failed: %w", err)
	}
	if status < 200 || status >= 300 {
		return Response{}, apiError("anthropic", status, body)
	}

	var aResp anthropicResponse
	if err := json.Unmarshal(body, &aResp); err != nil {
		return Response{}, fmt.Errorf("parse JSON response: %w", err)
	}
	if aResp.Error != nil {
		return Response{}, errors.New(aResp.Error.Message)
	}
	if len(aResp.Content) == 0 {
		return Response{}, errors.New("empty response from Anthropic")
	}

	var text strings.Builder
	for _, block := range aResp.Content {
		if block.Type == "text" {
			text.WriteString(block.Text)
		}
	}

	return parseSuggestionsJSON(text.String())
}