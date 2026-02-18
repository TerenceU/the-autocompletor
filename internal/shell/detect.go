package shell

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Shell string

const (
	Fish Shell = "fish"
	Bash Shell = "bash"
	Zsh  Shell = "zsh"
)

var supported = map[Shell]bool{
	Fish: true,
	Bash: true,
	Zsh:  true,
}

// Detect returns the current shell, checking env variables in order.
func Detect() (Shell, error) {
	// Fish sets $FISH_VERSION, zsh sets $ZSH_VERSION, bash sets $BASH_VERSION
	if os.Getenv("FISH_VERSION") != "" {
		return Fish, nil
	}
	if os.Getenv("ZSH_VERSION") != "" {
		return Zsh, nil
	}
	if os.Getenv("BASH_VERSION") != "" {
		return Bash, nil
	}

	// Fallback: parse $SHELL
	shellPath := os.Getenv("SHELL")
	if shellPath == "" {
		return "", fmt.Errorf("could not detect current shell: $SHELL is not set")
	}
	name := Shell(strings.ToLower(filepath.Base(shellPath)))
	if !supported[name] {
		return "", fmt.Errorf("shell %q is not supported (supported: fish, bash, zsh)", name)
	}
	return name, nil
}

// Parse validates and returns a Shell from a user-provided string.
func Parse(s string) (Shell, error) {
	sh := Shell(strings.ToLower(s))
	if !supported[sh] {
		return "", fmt.Errorf("shell %q is not supported (supported: fish, bash, zsh)", s)
	}
	return sh, nil
}

// CompletionsDir returns the default completions install directory for the shell.
func CompletionsDir(sh Shell) string {
	home, _ := os.UserHomeDir()
	switch sh {
	case Fish:
		return filepath.Join(home, ".config", "fish", "completions")
	case Bash:
		return filepath.Join(home, ".bash_completion.d")
	case Zsh:
		return filepath.Join(home, ".zsh", "completions")
	default:
		return ""
	}
}
