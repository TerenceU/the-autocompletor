package parser

import (
	"os/exec"
	"regexp"
	"strings"

	"github.com/terencetachiona/the-autocompletor/internal/model"
)

// splitLinePattern splits a help line into "flags part" and "description part"
// using 2+ spaces or a tab as separator. Handles up to 12 spaces of leading indent
// (man pages use 7, --help output typically uses 2-6).
var splitLinePattern = regexp.MustCompile(`^(\s{1,12}-[^\t]+?)(?:\s{2,}|\t)(.+)$`)

// longFlagPattern extracts long flags from the flags part.
// Handles --flag and --[no-]flag (git-style).
var longFlagPattern = regexp.MustCompile(`--(?:\[no-\])?([a-zA-Z0-9][a-zA-Z0-9\-]*)`)

// shortFlagPattern extracts short flags from the flags part.
var shortFlagPattern = regexp.MustCompile(`(?:^|[,\s])(-[a-zA-Z0-9])(?:[,\s]|$)`)

// takesArgPattern detects whether a flag takes a value.
var takesArgPattern = regexp.MustCompile(`(?i)(value|<[^>]+>|\[.*\]|file|path|string|int|num|port|url|host|addr|dir|name|key|secret|token)`)

// subcommandPattern matches lines in COMMANDS/SUBCOMMANDS sections.
var subcommandPattern = regexp.MustCompile(`^\s{2,4}([a-z][a-zA-Z0-9_\-]+)\s{2,}(.+)$`)

// flagOnlyPattern matches a flag line that has no description on the same line.
var flagOnlyPattern = regexp.MustCompile(`^\s{1,12}(-[a-zA-Z0-9,\s\-\[\]<>=]+?)$`)

// indentOf returns the number of leading spaces/tabs in a line.
func indentOf(line string) int {
	return len(line) - len(strings.TrimLeft(line, " \t"))
}

// extractFlags parses help output lines and returns all found flags.
// Handles single-line, man-page style (flag then description on next line),
// and continuation multi-line descriptions.
func extractFlags(lines []string) []model.Flag {
	var flags []model.Flag
	seen := map[string]bool{}

	// Pass 1: merge flag-only lines with their following description lines,
	// and join continuation lines that are very deeply indented.
	joined := make([]string, 0, len(lines))
	for i := 0; i < len(lines); i++ {
		line := lines[i]

		// If previous line was written to joined and this looks like a deep continuation
		if len(joined) > 0 &&
			len(line) > 20 &&
			strings.HasPrefix(line, "                    ") { // 20+ spaces
			joined[len(joined)-1] += " " + strings.TrimSpace(line)
			continue
		}

		// Check if this is a flag-only line (flag pattern but no inline description)
		trimmed := strings.TrimSpace(line)
		if trimmed != "" &&
			strings.HasPrefix(trimmed, "-") &&
			splitLinePattern.FindStringSubmatch(line) == nil {
			// Look ahead for a description line
			if i+1 < len(lines) {
				next := lines[i+1]
				nextTrimmed := strings.TrimSpace(next)
				nextIndent := indentOf(next)
				curIndent := indentOf(line)
				if nextTrimmed != "" &&
					!strings.HasPrefix(nextTrimmed, "-") &&
					nextIndent > curIndent {
					// Merge: flag + description from next line
					joined = append(joined, strings.TrimRight(line, " \t")+"    "+nextTrimmed)
					i++ // skip next line (already consumed)
					continue
				}
			}
		}

		joined = append(joined, line)
	}

	for _, line := range joined {
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

		if !strings.Contains(flagsPart, "-") {
			continue
		}

		longs := longFlagPattern.FindAllStringSubmatch(flagsPart, -1)
		shorts := shortFlagPattern.FindAllStringSubmatch(flagsPart, -1)

		if len(longs) == 0 && len(shorts) == 0 {
			continue
		}

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

// subEntry holds a subcommand name and its description from the help output.
type subEntry struct {
	name string
	desc string
}

// extractSubcommands finds subcommand names and descriptions from help output.
// Handles both explicit COMMANDS: sections and git-style unlabelled lists.
func extractSubcommands(lines []string) []subEntry {
	var subs []subEntry
	seen := map[string]bool{}
	inCommandsSection := false

	for _, line := range lines {
		low := strings.ToLower(strings.TrimSpace(line))

		if isCommandsHeader(low) {
			inCommandsSection = true
			continue
		}
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
				subs = append(subs, subEntry{name: name, desc: strings.TrimSpace(m[2])})
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

