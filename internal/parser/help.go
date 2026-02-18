package parser

import (
	"os/exec"
	"regexp"
	"strings"

	"github.com/terencetachiona/the-autocompletor/internal/model"
)

// splitLinePattern splits a help line into "flags part" and "description part"
// using 2+ spaces or a tab as separator.
var splitLinePattern = regexp.MustCompile(`^(\s{1,6}-[^\t]+?)(?:\s{2,}|\t)(.+)$`)

// longFlagPattern extracts long flags from the flags part.
var longFlagPattern = regexp.MustCompile(`--([a-zA-Z0-9][a-zA-Z0-9\-]*)`)

// shortFlagPattern extracts short flags from the flags part.
var shortFlagPattern = regexp.MustCompile(`(?:^|[,\s])(-[a-zA-Z0-9])(?:[,\s]|$)`)

// takesArgPattern detects whether a flag takes a value.
var takesArgPattern = regexp.MustCompile(`(?i)(value|<[^>]+>|\[.*\]|file|path|string|int|num|port|url|host|addr|dir|name|key|secret|token)`)

// subcommandPattern matches lines in COMMANDS/SUBCOMMANDS sections.
var subcommandPattern = regexp.MustCompile(`^\s{2,4}([a-z][a-zA-Z0-9_\-]+)\s{2,}(.+)$`)

// extractFlags parses help output lines and returns all found flags.
func extractFlags(lines []string) []model.Flag {
	var flags []model.Flag
	seen := map[string]bool{}

	for _, line := range lines {
		// Must start with whitespace followed by a dash (flag line)
		if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			continue
		}

		m := splitLinePattern.FindStringSubmatch(line)
		if m == nil {
			continue
		}

		flagsPart := m[1]
		desc := strings.TrimSpace(m[2])

		// Skip if flagsPart doesn't contain a dash
		if !strings.Contains(flagsPart, "-") {
			continue
		}

		longs := longFlagPattern.FindAllStringSubmatch(flagsPart, -1)
		shorts := shortFlagPattern.FindAllStringSubmatch(flagsPart, -1)

		if len(longs) == 0 && len(shorts) == 0 {
			continue
		}

		// Use the first long flag as the key
		long := ""
		if len(longs) > 0 {
			long = "--" + longs[0][1]
		}
		short := ""
		if len(shorts) > 0 {
			short = shorts[0][1]
		}

		key := long
		if key == "" {
			key = short
		}
		if seen[key] {
			continue
		}
		seen[key] = true

		flags = append(flags, model.Flag{
			Short:       short,
			Long:        long,
			Description: desc,
			TakesArg:    takesArgPattern.MatchString(flagsPart),
		})
	}

	return flags
}

// extractSubcommands finds subcommand names from help output.
// Handles both explicit COMMANDS: sections and git-style unlabelled lists.
func extractSubcommands(lines []string) []string {
	var subs []string
	seen := map[string]bool{}
	inCommandsSection := false

	for _, line := range lines {
		low := strings.ToLower(strings.TrimSpace(line))

		if isCommandsHeader(low) {
			inCommandsSection = true
			continue
		}
		// A non-indented non-empty line that isn't a header resets the section
		if inCommandsSection && line != "" && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			inCommandsSection = false
		}

		if inCommandsSection || looksLikeSubcommandLine(line) {
			if m := subcommandPattern.FindStringSubmatch(line); m != nil {
				name := m[1]
				if isReservedWord(name) || seen[name] {
					continue
				}
				seen[name] = true
				subs = append(subs, name)
			}
		}
	}

	return subs
}

// looksLikeSubcommandLine returns true for lines with 3-4 spaces indent,
// a short lowercase word, then 2+ spaces and a description (git-style).
func looksLikeSubcommandLine(line string) bool {
	return subcommandPattern.MatchString(line)
}

func isOptionsHeader(s string) bool {
	return strings.HasPrefix(s, "option") ||
		strings.HasPrefix(s, "flag") ||
		strings.HasPrefix(s, "global option") ||
		strings.HasPrefix(s, "available option")
}

func isCommandsHeader(s string) bool {
	return s == "commands:" ||
		s == "subcommands:" ||
		s == "available commands:" ||
		strings.HasPrefix(s, "command")
}

func isReservedWord(s string) bool {
	reserved := map[string]bool{
		"help": true, "version": true, "completion": true,
	}
	return reserved[s]
}

// parseManPage runs man and returns a Command with flags parsed from the man page.
func parseManPage(program string) (*model.Command, error) {
	out, err := exec.Command("sh", "-c", "man "+program+" 2>/dev/null | col -bx").Output()
	if err != nil || len(out) == 0 {
		return nil, err
	}

	lines := strings.Split(string(out), "\n")
	cmd := &model.Command{Name: program}
	cmd.Flags = extractFlags(lines)
	return cmd, nil
}

