package parser

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/terencetachiona/the-autocompletor/internal/model"
)

const maxDepth = 3

// Parse builds a Command tree for the given program by trying:
// 1. man page
// 2. --help output + recursive subcommand discovery
func Parse(program string) (*model.Command, error) {
	// Try man page first
	manCmd, err := parseManPage(program)
	if err == nil && manCmd != nil && len(manCmd.Flags) > 0 {
		// Still run help parser to discover subcommands not in man page
		helpCmd, herr := ParseHelp(program)
		if herr == nil {
			mergeSubcommands(manCmd, helpCmd)
		}
		return manCmd, nil
	}

	// Fallback to --help
	return ParseHelp(program)
}

// ParseHelp parses --help output for the given command path and recurses into subcommands.
func ParseHelp(args ...string) (*model.Command, error) {
	return parseHelpRecursive(args, 0)
}

func parseHelpRecursive(args []string, depth int) (*model.Command, error) {
	if depth > maxDepth {
		return nil, fmt.Errorf("max depth reached")
	}

	program := args[0]
	output, err := runHelp(args...)
	if err != nil {
		return nil, fmt.Errorf("could not get help for %q: %w", strings.Join(args, " "), err)
	}

	cmd := &model.Command{Name: program}
	if len(args) > 1 {
		cmd.Name = args[len(args)-1]
	}

	lines := strings.Split(output, "\n")
	cmd.Flags = extractFlags(lines)
	subNames := extractSubcommands(lines)

	for _, sub := range subNames {
		subArgs := append(args, sub)
		subCmd, err := parseHelpRecursive(subArgs, depth+1)
		if err != nil {
			// Best effort: add subcommand without flags
			cmd.Subcommands = append(cmd.Subcommands, &model.Command{Name: sub})
			continue
		}
		cmd.Subcommands = append(cmd.Subcommands, subCmd)
	}

	return cmd, nil
}

// runHelp executes <args> --help, falling back to -h, with a timeout and pager disabled.
func runHelp(args ...string) (string, error) {
	for _, flag := range []string{"--help", "-h"} {
		cmdArgs := append(args[1:], flag)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		cmd := exec.CommandContext(ctx, args[0], cmdArgs...)
		// Disable pagers so programs like git don't block waiting for interaction
		cmd.Env = append(os.Environ(),
			"PAGER=cat",
			"GIT_PAGER=cat",
			"MANPAGER=cat",
			"TERM=dumb",
			"GIT_TERMINAL_PROMPT=0",
		)
		out, _ := cmd.CombinedOutput()
		cancel()

		if len(out) > 0 {
			return string(out), nil
		}
	}
	return "", fmt.Errorf("no help output for %q", strings.Join(args, " "))
}

// mergeSubcommands copies subcommands from src into dst if not already present.
func mergeSubcommands(dst, src *model.Command) {
	existing := map[string]bool{}
	for _, s := range dst.Subcommands {
		existing[s.Name] = true
	}
	for _, s := range src.Subcommands {
		if !existing[s.Name] {
			dst.Subcommands = append(dst.Subcommands, s)
		}
	}
}
