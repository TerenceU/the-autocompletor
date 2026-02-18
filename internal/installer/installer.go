package installer

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/terencetachiona/the-autocompletor/internal/shell"
)

// completionsFileName returns the correct filename for the given shell.
func completionsFileName(sh shell.Shell, program string) string {
	switch sh {
	case shell.Zsh:
		return "_" + program
	default:
		return program + "." + string(sh)
	}
}

// Install writes the completion content to the correct directory for the shell.
func Install(sh shell.Shell, program, content string) (string, error) {
	dir := shell.CompletionsDir(sh)
	if dir == "" {
		return "", fmt.Errorf("unknown install directory for shell %q", sh)
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("could not create completions directory %q: %w", dir, err)
	}

	fileName := completionsFileName(sh, program)
	path := filepath.Join(dir, fileName)

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("could not write completions file: %w", err)
	}

	return path, nil
}
