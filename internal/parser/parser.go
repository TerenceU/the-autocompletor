package parser

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/TerenceU/the-autocompletor/internal/model"
)

const maxDepth = 3

// ProgressFunc is called whenever the parser starts processing a command.
// Callers can use it to display progress (e.g. print to stderr).
type ProgressFunc func(msg string)

// Parse builds a Command tree for the given program by trying:
// 1. man page
// 2. --help output + recursive subcommand discovery
func Parse(program string) (*model.Command, error) {
	return ParseWithProgress(program, nil)
}

// ParseWithProgress is like Parse but calls progress for each step.
func ParseWithProgress(program string, progress ProgressFunc) (*model.Command, error) {
	notify := func(msg string) {
		if progress != nil {
			progress(msg)
		}
	}

	notify(fmt.Sprintf("reading man page for %q", program))
	manCmd, err := parseManPage(program)
	if err == nil && manCmd != nil && len(manCmd.Flags) > 0 {
		notify(fmt.Sprintf("reading --help for %q", program))
		helpCmd, herr := ParseHelp(program)
		if herr == nil {
			mergeSubcommands(manCmd, helpCmd)
		}
		for _, sub := range manCmd.Subcommands {
			if len(sub.Flags) == 0 {
				notify(fmt.Sprintf("reading --help for %q %q", program, sub.Name))
				if subHelp, serr := ParseHelp(program, sub.Name); serr == nil {
					sub.Flags = subHelp.Flags
					if sub.Description == "" {
						sub.Description = subHelp.Description
					}
				}
			}
		}
		return manCmd, nil
	}

	notify(fmt.Sprintf("reading --help for %q", program))
	return parseHelpRecursive([]string{program}, 0, "", notify)
}

// ParseHelp parses --help output for the given command path and recurses into subcommands.
func ParseHelp(args ...string) (*model.Command, error) {
	return parseHelpRecursive(args, 0, "", nil)
}

// parseHelpRecursive recurses into subcommands.
// parentOutput is the help output of the parent call; if a child returns the
// same output we stop recursing (the program doesn't support per-subcommand help).
func parseHelpRecursive(args []string, depth int, parentOutput string, progress ProgressFunc) (*model.Command, error) {
	if depth > maxDepth {
		return nil, fmt.Errorf("max depth reached")
	}

	if progress != nil {
		progress(fmt.Sprintf("reading --help for %q", strings.Join(args, " ")))
	}

	program := args[0]
	output, err := runHelp(args...)
	if err != nil {
		return nil, fmt.Errorf("could not get help for %q: %w", strings.Join(args, " "), err)
	}

	// If the output is identical to the parent's, this program doesn't have
	// per-subcommand help — stop recursing to avoid combinatorial explosion.
	if parentOutput != "" && normalizeHelp(output) == normalizeHelp(parentOutput) {
		cmd := &model.Command{Name: args[len(args)-1]}
		return cmd, nil
	}

	cmd := &model.Command{Name: program}
	if len(args) > 1 {
		cmd.Name = args[len(args)-1]
	}

	lines := strings.Split(output, "\n")
	cmd.Flags = extractFlags(lines)
	subEntries := extractSubcommands(lines, false)

	if len(subEntries) == 0 {
		return cmd, nil
	}

	// Resolve subcommands in parallel (bounded to 6 workers).
	type result struct {
		index int
		cmd   *model.Command
		err   error
	}

	results := make([]result, len(subEntries))
	sem := make(chan struct{}, 6)
	done := make(chan result, len(subEntries))

	for i, entry := range subEntries {
		i, entry := i, entry
		sem <- struct{}{}
		go func() {
			defer func() { <-sem }()
			subArgs := append(append([]string{}, args...), entry.name)
			subCmd, serr := parseHelpRecursive(subArgs, depth+1, output, progress)
			done <- result{index: i, cmd: subCmd, err: serr}
		}()
	}

	for range subEntries {
		r := <-done
		results[r.index] = r
	}

	// Assemble in original order
	for i, entry := range subEntries {
		r := results[i]
		if r.err != nil {
			cmd.Subcommands = append(cmd.Subcommands, &model.Command{
				Name:        entry.name,
				Description: entry.desc,
			})
			continue
		}
		if r.cmd.Description == "" {
			r.cmd.Description = entry.desc
		}
		cmd.Subcommands = append(cmd.Subcommands, r.cmd)
	}

	return cmd, nil
}

// normalizeHelp strips leading/trailing whitespace for comparison.
func normalizeHelp(s string) string {
	return strings.TrimSpace(s)
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
