package providers

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/terbash/terbash/pkg/types"
)

type OllamaProvider struct {
	*BaseProvider
}

func NewOllamaProvider(config types.ProviderConfig) *OllamaProvider {
	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	return &OllamaProvider{
		BaseProvider: NewBaseProvider(config, baseURL, nil),
	}
}

func (p *OllamaProvider) ChatCompletion(req types.ChatCompletionRequest) (*types.ChatCompletionResponse, error) {
	body, err := p.doRequest(context.Background(), "POST", "/api/chat", map[string]interface{}{
		"model":    req.Model,
		"messages": req.Messages,
		"tools":    req.Tools,
		"stream":   false,
		"options": map[string]interface{}{
			"temperature": req.Temperature,
			"num_predict": req.MaxTokens,
		},
	})
	if err != nil {
		return nil, err
	}

	var resp struct {
		Model     string      `json:"model"`
		Message   types.Message `json:"message"`
		Done      bool        `json:"done"`
		TotalDuration int64   `json:"total_duration"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &types.ChatCompletionResponse{
		Model: resp.Model,
		Choices: []types.Choice{
			{
				Index: 0,
				Message: resp.Message,
				FinishReason: "stop",
			},
		},
	}, nil
}

func (p *OllamaProvider) ChatCompletionStream(req types.ChatCompletionRequest) (<-chan types.StreamChunk, error) {
	ch := make(chan types.StreamChunk)

	resp, err := p.doStreamRequest(context.Background(), "POST", "/api/chat", map[string]interface{}{
		"model":    req.Model,
		"messages": req.Messages,
		"tools":    req.Tools,
		"stream":   true,
		"options": map[string]interface{}{
			"temperature": req.Temperature,
			"num_predict": req.MaxTokens,
		},
	})
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
				Message types.Message `json:"message"`
				Done    bool          `json:"done"`
			}
			if err := json.Unmarshal([]byte(line), &chunk); err != nil {
				continue
			}

			ch <- types.StreamChunk{
				Model: req.Model,
				Choices: []types.StreamChoice{
					{
						Index: 0,
						Delta: chunk.Message,
						FinishReason: map[bool]string{true: "stop", false: ""}[chunk.Done],
					},
				},
			}

			if chunk.Done {
				break
			}
		}
	}()

	return ch, nil
}

func (p *OllamaProvider) GetModels() ([]string, error) {
	body, err := p.doRequest(context.Background(), "GET", "/api/tags", nil)
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

	models := make([]string, len(resp.Models))
	for i, m := range resp.Models {
		models[i] = m.Name
	}
	return models, nil
}

func (p *OllamaProvider) Close() error {
	return nil
}

func (p *OllamaProvider) GetConfig() types.ProviderConfig {
	return p.BaseProvider.GetConfig()
}