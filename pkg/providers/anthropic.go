package providers

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/terbash/terbash/pkg/types"
)

type AnthropicProvider struct {
	*BaseProvider
}

func NewAnthropicProvider(config types.ProviderConfig) *AnthropicProvider {
	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = "https://api.anthropic.com/v1"
	}
	headers := map[string]string{
		"anthropic-version": "2023-06-01",
		"x-api-key":         config.APIKey,
	}
	return &AnthropicProvider{
		BaseProvider: NewBaseProvider(config, baseURL, headers),
	}
}

func (p *AnthropicProvider) ChatCompletion(req types.ChatCompletionRequest) (*types.ChatCompletionResponse, error) {
	anthropicReq := p.convertRequest(req)
	body, err := p.doRequest(context.Background(), "POST", "/messages", anthropicReq)
	if err != nil {
		return nil, err
	}

	var anthropicResp struct {
		ID      string `json:"id"`
		Type    string `json:"type"`
		Role    string `json:"role"`
		Content []struct {
			Type  string `json:"type"`
			Text  string `json:"text"`
			ID    string `json:"id,omitempty"`
			Name  string `json:"name,omitempty"`
			Input json.RawMessage `json:"input,omitempty"`
		} `json:"content"`
		Model       string `json:"model"`
		StopReason  string `json:"stop_reason"`
		Usage       struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &anthropicResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	var toolCalls []types.ToolCall
	var content string
	for _, c := range anthropicResp.Content {
		if c.Type == "text" {
			content = c.Text
		} else if c.Type == "tool_use" {
			toolCalls = append(toolCalls, types.ToolCall{
				ID: c.ID,
				Type: "function",
				Function: types.FunctionCall{
					Name:      c.Name,
					Arguments: string(c.Input),
				},
			})
		}
	}

	return &types.ChatCompletionResponse{
		ID:      anthropicResp.ID,
		Model:   anthropicResp.Model,
		Choices: []types.Choice{{Index: 0, Message: types.Message{Role: "assistant", Content: content, ToolCalls: toolCalls}, FinishReason: anthropicResp.StopReason}},
		Usage: types.Usage{
			PromptTokens:     anthropicResp.Usage.InputTokens,
			CompletionTokens: anthropicResp.Usage.OutputTokens,
			TotalTokens:      anthropicResp.Usage.InputTokens + anthropicResp.Usage.OutputTokens,
		},
	}, nil
}

func (p *AnthropicProvider) ChatCompletionStream(req types.ChatCompletionRequest) (<-chan types.StreamChunk, error) {
	anthropicReq := p.convertRequest(req)
	anthropicReq["stream"] = true
	ch := make(chan types.StreamChunk)

	resp, err := p.doStreamRequest(context.Background(), "POST", "/messages", anthropicReq)
	if err != nil {
		close(ch)
		return ch, err
	}

	go func() {
		defer close(ch)
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				break
			}

			var chunk struct {
				Type  string `json:"type"`
				Delta struct {
					Type  string `json:"type"`
					Text  string `json:"text"`
					ID    string `json:"id,omitempty"`
					Name  string `json:"name,omitempty"`
					Input json.RawMessage `json:"input,omitempty"`
				} `json:"delta"`
			}
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				continue
			}

			var deltaMsg types.Message
			if chunk.Delta.Type == "text_delta" {
				deltaMsg.Content = chunk.Delta.Text
			} else if chunk.Delta.Type == "tool_use" {
				deltaMsg.ToolCalls = []types.ToolCall{{
					ID: chunk.Delta.ID,
					Type: "function",
					Function: types.FunctionCall{
						Name:      chunk.Delta.Name,
						Arguments: string(chunk.Delta.Input),
					},
				}}
			}

			ch <- types.StreamChunk{
				Model: req.Model,
				Choices: []types.StreamChoice{{Index: 0, Delta: deltaMsg}},
			}
		}
	}()

	return ch, nil
}

func (p *AnthropicProvider) GetModels() ([]string, error) {
	return []string{"claude-3-opus-20240229", "claude-3-sonnet-20240229", "claude-3-haiku-20240307"}, nil
}

func (p *AnthropicProvider) Close() error {
	return nil
}

func (p *AnthropicProvider) convertRequest(req types.ChatCompletionRequest) map[string]interface{} {
	messages := make([]map[string]interface{}, 0, len(req.Messages))
	var systemPrompt string

	for _, m := range req.Messages {
		if m.Role == "system" {
			systemPrompt = m.Content
			continue
		}
		msg := map[string]interface{}{
			"role":    m.Role,
			"content": m.Content,
		}
		if len(m.ToolCalls) > 0 {
			content := make([]map[string]interface{}, 0, len(m.ToolCalls))
			for _, tc := range m.ToolCalls {
				content = append(content, map[string]interface{}{
					"type":  "tool_use",
					"id":    tc.ID,
					"name":  tc.Function.Name,
					"input": json.RawMessage(tc.Function.Arguments),
				})
			}
			msg["content"] = content
		}
		messages = append(messages, msg)
	}

	tools := make([]map[string]interface{}, 0, len(req.Tools))
	for _, t := range req.Tools {
		tools = append(tools, map[string]interface{}{
			"name":        t.Function.Name,
			"description": t.Function.Description,
			"input_schema": t.Function.Parameters,
		})
	}

	return map[string]interface{}{
		"model":       req.Model,
		"messages":    messages,
		"system":      systemPrompt,
		"tools":       tools,
		"temperature": req.Temperature,
		"max_tokens":  req.MaxTokens,
	}
}