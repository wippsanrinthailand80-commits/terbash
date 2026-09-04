package providers

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/terbash/terbash/pkg/types"
)

type OpenAICompatibleProvider struct {
	*BaseProvider
}

func NewOpenAICompatibleProvider(config types.ProviderConfig, baseURL string) *OpenAICompatibleProvider {
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	return &OpenAICompatibleProvider{
		BaseProvider: NewBaseProvider(config, baseURL, nil),
	}
}

func (p *OpenAICompatibleProvider) ChatCompletion(req types.ChatCompletionRequest) (*types.ChatCompletionResponse, error) {
	body, err := p.doRequest(context.Background(), "POST", "/chat/completions", req)
	if err != nil {
		return nil, err
	}

	var resp types.ChatCompletionResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &resp, nil
}

func (p *OpenAICompatibleProvider) ChatCompletionStream(req types.ChatCompletionRequest) (<-chan types.StreamChunk, error) {
	req.Stream = true
	ch := make(chan types.StreamChunk)

	resp, err := p.doStreamRequest(context.Background(), "POST", "/chat/completions", req)
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

			var chunk types.StreamChunk
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				continue
			}
			ch <- chunk
		}
	}()

	return ch, nil
}

func (p *OpenAICompatibleProvider) GetModels() ([]string, error) {
	body, err := p.doRequest(context.Background(), "GET", "/models", nil)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	models := make([]string, len(resp.Data))
	for i, m := range resp.Data {
		models[i] = m.ID
	}
	return models, nil
}

func (p *OpenAICompatibleProvider) Close() error {
	return nil
}

func (p *OpenAICompatibleProvider) GetConfig() types.ProviderConfig {
	return p.BaseProvider.GetConfig()
}

func NewOpenAIProvider(config types.ProviderConfig) *OpenAICompatibleProvider {
	return NewOpenAICompatibleProvider(config, "https://api.openai.com/v1")
}

func NewGroqProvider(config types.ProviderConfig) *OpenAICompatibleProvider {
	return NewOpenAICompatibleProvider(config, "https://api.groq.com/openai/v1")
}

func NewMistralProvider(config types.ProviderConfig) *OpenAICompatibleProvider {
	return NewOpenAICompatibleProvider(config, "https://api.mistral.ai/v1")
}

func NewCohereProvider(config types.ProviderConfig) *OpenAICompatibleProvider {
	return NewOpenAICompatibleProvider(config, "https://api.cohere.ai/v1")
}

func NewTogetherProvider(config types.ProviderConfig) *OpenAICompatibleProvider {
	return NewOpenAICompatibleProvider(config, "https://api.together.xyz/v1")
}

func NewPerplexityProvider(config types.ProviderConfig) *OpenAICompatibleProvider {
	return NewOpenAICompatibleProvider(config, "https://api.perplexity.ai")
}

func NewDeepSeekProvider(config types.ProviderConfig) *OpenAICompatibleProvider {
	return NewOpenAICompatibleProvider(config, "https://api.deepseek.com/v1")
}

func NewXAIProvider(config types.ProviderConfig) *OpenAICompatibleProvider {
	return NewOpenAICompatibleProvider(config, "https://api.x.ai/v1")
}

func NewCustomProvider(config types.ProviderConfig) *OpenAICompatibleProvider {
	return NewOpenAICompatibleProvider(config, config.BaseURL)
}

// NewLlamaCppProvider talks to a local llama.cpp server (llama-server),
// which exposes an OpenAI-compatible API. Pure C/C++ inference with no
// Ollama/Go overhead - ideal for Termux. Override the port via base_url.
func NewLlamaCppProvider(config types.ProviderConfig) *OpenAICompatibleProvider {
	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = "http://localhost:8080/v1"
	}
	return NewOpenAICompatibleProvider(config, baseURL)
}