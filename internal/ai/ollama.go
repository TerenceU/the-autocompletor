package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/terencetachiona/the-autocompletor/internal/model"
	"github.com/terencetachiona/the-autocompletor/internal/shell"
)

const defaultOllamaURL = "http://localhost:11434"
const defaultOllamaModel = "llama3"

// OllamaOptions configures the Ollama AI backend.
type OllamaOptions struct {
	BaseURL string
	Model   string
}

// Ollama asks a local Ollama instance to generate completions for the program.
func Ollama(program string, sh shell.Shell, opts OllamaOptions) (*model.Command, error) {
	if opts.BaseURL == "" {
		opts.BaseURL = defaultOllamaURL
	}
	if opts.Model == "" {
		opts.Model = defaultOllamaModel
	}

	prompt := buildPrompt(program, sh)

	body, _ := json.Marshal(map[string]any{
		"model":  opts.Model,
		"prompt": prompt,
		"stream": false,
	})

	resp, err := http.Post(opts.BaseURL+"/api/generate", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("ollama request failed: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)

	var result struct {
		Response string `json:"response"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("could not parse ollama response: %w", err)
	}

	return parseAIResponse(program, result.Response), nil
}

func buildPrompt(program string, sh shell.Shell) string {
	return fmt.Sprintf(`You are a shell completion expert. Generate a list of CLI flags and subcommands for the program "%s".

Respond ONLY in this exact format, one per line:
FLAG|short|long|description|takes_arg
SUBCOMMAND|name|description

Where:
- short is the short flag like -u (or empty)
- long is the long flag like --url (or empty)
- takes_arg is true or false
- For SUBCOMMAND lines, name is the subcommand name

Example:
FLAG|-u|--url|The target URL|true
FLAG|-v||Enable verbose output|false
SUBCOMMAND|dir|Directory enumeration mode

Now generate completions for: %s
Target shell: %s`, program, program, sh)
}

// parseAIResponse parses the structured AI output into a Command tree.
func parseAIResponse(program, response string) *model.Command {
	cmd := &model.Command{Name: program}

	for _, line := range strings.Split(response, "\n") {
		line = strings.TrimSpace(line)
		parts := strings.Split(line, "|")

		switch {
		case strings.HasPrefix(line, "FLAG|") && len(parts) == 5:
			cmd.Flags = append(cmd.Flags, model.Flag{
				Short:       parts[1],
				Long:        parts[2],
				Description: parts[3],
				TakesArg:    parts[4] == "true",
			})
		case strings.HasPrefix(line, "SUBCOMMAND|") && len(parts) == 3:
			cmd.Subcommands = append(cmd.Subcommands, &model.Command{
				Name:        parts[1],
				Description: parts[2],
			})
		}
	}

	return cmd
}
