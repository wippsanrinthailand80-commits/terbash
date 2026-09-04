package providers

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/terbash/terbash/pkg/types"
)

type GeminiProvider struct {
	*BaseProvider
}

func NewGeminiProvider(config types.ProviderConfig) *GeminiProvider {
	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = "https://generativelanguage.googleapis.com/v1beta"
	}
	return &GeminiProvider{
		BaseProvider: NewBaseProvider(config, baseURL, nil),
	}
}

func (p *GeminiProvider) ChatCompletion(req types.ChatCompletionRequest) (*types.ChatCompletionResponse, error) {
	geminiReq := p.convertRequest(req)
	endpoint := fmt.Sprintf("/models/%s:generateContent?key=%s", req.Model, p.config.APIKey)
	body, err := p.doRequest(context.Background(), "POST", endpoint, geminiReq)
	if err != nil {
		return nil, err
	}

	var geminiResp struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text       string          `json:"text"`
					FunctionCall *FunctionCall `json:"functionCall,omitempty"`
				} `json:"parts"`
			} `json:"content"`
			FinishReason string `json:"finishReason"`
		} `json:"candidates"`
		UsageMetadata struct {
			PromptTokenCount     int `json:"promptTokenCount"`
			CandidatesTokenCount int `json:"candidatesTokenCount"`
			TotalTokenCount      int `json:"totalTokenCount"`
		} `json:"usageMetadata"`
	}
	if err := json.Unmarshal(body, &geminiResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if len(geminiResp.Candidates) == 0 {
		return nil, fmt.Errorf("no candidates in response")
	}

	candidate := geminiResp.Candidates[0]
	var content string
	var toolCalls []types.ToolCall
	for _, part := range candidate.Content.Parts {
		if part.Text != "" {
			content += part.Text
		}
		if part.FunctionCall != nil {
			args, _ := json.Marshal(part.FunctionCall.Args)
			toolCalls = append(toolCalls, types.ToolCall{
				ID: fmt.Sprintf("call_%d", len(toolCalls)),
				Type: "function",
				Function: types.FunctionCall{
					Name:      part.FunctionCall.Name,
					Arguments: string(args),
				},
			})
		}
	}

	return &types.ChatCompletionResponse{
		Model: req.Model,
		Choices: []types.Choice{
			{Index: 0, Message: types.Message{Role: "model", Content: content, ToolCalls: toolCalls}, FinishReason: candidate.FinishReason},
		},
		Usage: types.Usage{
			PromptTokens:     geminiResp.UsageMetadata.PromptTokenCount,
			CompletionTokens: geminiResp.UsageMetadata.CandidatesTokenCount,
			TotalTokens:      geminiResp.UsageMetadata.TotalTokenCount,
		},
	}, nil
}

func (p *GeminiProvider) ChatCompletionStream(req types.ChatCompletionRequest) (<-chan types.StreamChunk, error) {
	geminiReq := p.convertRequest(req)
	endpoint := fmt.Sprintf("/models/%s:streamGenerateContent?key=%s", req.Model, p.config.APIKey)
	ch := make(chan types.StreamChunk)

	resp, err := p.doStreamRequest(context.Background(), "POST", endpoint, geminiReq)
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

			var chunk struct {
				Candidates []struct {
					Content struct {
						Parts []struct {
							Text       string          `json:"text"`
							FunctionCall *FunctionCall `json:"functionCall,omitempty"`
						} `json:"parts"`
					} `json:"content"`
				} `json:"candidates"`
			}
			if err := json.Unmarshal([]byte(line), &chunk); err != nil {
				continue
			}

			if len(chunk.Candidates) > 0 && len(chunk.Candidates[0].Content.Parts) > 0 {
				part := chunk.Candidates[0].Content.Parts[0]
				delta := types.Message{}
				if part.Text != "" {
					delta.Content = part.Text
				}
				if part.FunctionCall != nil {
					args, _ := json.Marshal(part.FunctionCall.Args)
					delta.ToolCalls = []types.ToolCall{{
						ID: fmt.Sprintf("call_%d", 0),
						Type: "function",
						Function: types.FunctionCall{
							Name:      part.FunctionCall.Name,
							Arguments: string(args),
						},
					}}
				}
				ch <- types.StreamChunk{
					Model: req.Model,
					Choices: []types.StreamChoice{{Index: 0, Delta: delta}},
				}
			}
		}
	}()

	return ch, nil
}

func (p *GeminiProvider) GetModels() ([]string, error) {
	endpoint := fmt.Sprintf("/models?key=%s", p.config.APIKey)
	body, err := p.doRequest(context.Background(), "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	models := make([]string, 0, len(resp.Models))
	for _, m := range resp.Models {
		name := strings.TrimPrefix(m.Name, "models/")
		if strings.Contains(name, "gemini") {
			models = append(models, name)
		}
	}
	return models, nil
}

func (p *GeminiProvider) Close() error {
	return nil
}

func (p *GeminiProvider) GetConfig() types.ProviderConfig {
	return p.BaseProvider.GetConfig()
}

type FunctionCall struct {
	Name string                 `json:"name"`
	Args map[string]interface{} `json:"args"`
}

func (p *GeminiProvider) convertRequest(req types.ChatCompletionRequest) map[string]interface{} {
	contents := make([]map[string]interface{}, 0, len(req.Messages))
	for _, m := range req.Messages {
		role := "user"
		if m.Role == "assistant" || m.Role == "model" {
			role = "model"
		}
		if m.Role == "system" {
			continue
		}
		parts := []map[string]interface{}{{"text": m.Content}}
		for _, tc := range m.ToolCalls {
			var args map[string]interface{}
			json.Unmarshal([]byte(tc.Function.Arguments), &args)
			parts = append(parts, map[string]interface{}{
				"functionCall": map[string]interface{}{
					"name": tc.Function.Name,
					"args": args,
				},
			})
		}
		contents = append(contents, map[string]interface{}{
			"role":  role,
			"parts": parts,
		})
	}

	tools := make([]map[string]interface{}, 0, len(req.Tools))
	for _, t := range req.Tools {
		tools = append(tools, map[string]interface{}{
			"functionDeclarations": []map[string]interface{}{
				{
					"name":        t.Function.Name,
					"description": t.Function.Description,
					"parameters":  t.Function.Parameters,
				},
			},
		})
	}

	return map[string]interface{}{
		"contents":         contents,
		"tools":            tools,
		"generationConfig": map[string]interface{}{"temperature": req.Temperature, "maxOutputTokens": req.MaxTokens},
	}
}