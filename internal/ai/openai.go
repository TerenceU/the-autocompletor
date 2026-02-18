package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/terencetachiona/the-autocompletor/internal/model"
	"github.com/terencetachiona/the-autocompletor/internal/shell"
)

const openAIURL = "https://api.openai.com/v1/chat/completions"
const defaultOpenAIModel = "gpt-4o-mini"

// OpenAIOptions configures the OpenAI API backend.
type OpenAIOptions struct {
	APIKey string
	Model  string
}

// OpenAI asks the OpenAI API to generate completions for the program.
func OpenAI(program string, sh shell.Shell, opts OpenAIOptions) (*model.Command, error) {
	apiKey := opts.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("OpenAI API key not provided (use --api-key or set OPENAI_API_KEY)")
	}
	if opts.Model == "" {
		opts.Model = defaultOpenAIModel
	}

	prompt := buildPrompt(program, sh)

	body, _ := json.Marshal(map[string]any{
		"model": opts.Model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	})

	req, _ := http.NewRequest("POST", openAIURL, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("OpenAI request failed: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("could not parse OpenAI response: %w", err)
	}
	if result.Error != nil {
		return nil, fmt.Errorf("OpenAI error: %s", result.Error.Message)
	}
	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("OpenAI returned no choices")
	}

	return parseAIResponse(program, result.Choices[0].Message.Content), nil
}
