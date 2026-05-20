package parser

import (
	"os/exec"
	"regexp"
	"strings"

	"github.com/TerenceU/the-autocompletor/internal/model"
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

// subcommandNamePattern matches a valid subcommand name (1+ lowercase word with optional hyphens).
var subcommandNamePattern = regexp.MustCompile(`^[a-z][a-zA-Z0-9_\-]*$`)

// twoSpacesSplit splits a string on 2+ consecutive spaces (used to separate args from description).
var twoSpacesSplit = regexp.MustCompile(`\s{2,}`)

// subEntry holds a subcommand name and its description from the help output.
type subEntry struct {
	name string
	desc string
}

// extractSubcommands finds subcommand names and descriptions from help output.
// Handles:
//   - --help style: "    name [args]   description on same line"
//   - man page style: "       name [args]" with description on next indented line
//   - git-style unlabelled lists
//
// When strict=true, only explicit COMMANDS/SUBCOMMANDS section headers are trusted
// (safe for man pages). When strict=false, heuristic detection is also used
// (suitable for --help output which is generally cleaner).
func extractSubcommands(lines []string, strict bool) []subEntry {
	var subs []subEntry
	seen := map[string]bool{}
	inCommandsSection := false
	sectionIndent := -1 // indent level of subcommand entries in current section

	for i, line := range lines {
		low := strings.ToLower(strings.TrimSpace(line))

		// Section headers must have minimal indent (≤ 4 spaces) to avoid
		// matching prose lines like "            command. If --help..."
		if indentOf(line) <= 4 && isCommandsHeader(low) {
			inCommandsSection = true
			sectionIndent = -1
			continue
		}
		// Non-indented non-empty line ends the section
		if inCommandsSection && line != "" && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			inCommandsSection = false
			sectionIndent = -1
		}

		if !inCommandsSection && (strict || !looksLikeSubcommandLine(line)) {
			continue
		}

		lineIndent := indentOf(line)
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		if inCommandsSection {
			// Calibrate indent on first subcommand entry
			if sectionIndent == -1 {
				sectionIndent = lineIndent
			}
			// Lines more indented than section indent are descriptions/continuations
			if lineIndent > sectionIndent {
				continue
			}

			// Split on 2+ spaces to separate "name [args]" from "description"
			parts := twoSpacesSplit.Split(trimmed, 2)
			firstWord := strings.SplitN(parts[0], " ", 2)[0]

			if !subcommandNamePattern.MatchString(firstWord) || isReservedWord(firstWord) || seen[firstWord] {
				continue
			}
			seen[firstWord] = true

			desc := ""
			if len(parts) > 1 {
				desc = strings.TrimSpace(parts[1])
			}
			// If no inline description, look at next more-indented line
			if desc == "" && i+1 < len(lines) {
				next := strings.TrimSpace(lines[i+1])
				if next != "" && indentOf(lines[i+1]) > lineIndent && !strings.HasPrefix(next, "-") {
					// Trim at first period to keep it short
					if idx := strings.Index(next, ". "); idx != -1 {
						desc = next[:idx+1]
					} else {
						desc = next
					}
				}
			}

			subs = append(subs, subEntry{name: firstWord, desc: desc})
		} else {
			// Outside COMMANDS section: use strict pattern (avoids false positives)
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
	// s is already trimmed and lowercased.
	// Accept only known header patterns. Using "commands" (plural) avoids
	// matching prose lines like "command. If --help..." or "command(<action>)".
	return s == "commands" ||
		s == "commands:" ||
		s == "command:" ||
		s == "subcommands" ||
		s == "subcommands:" ||
		s == "available commands:" ||
		strings.HasPrefix(s, "commands ")
}

func isReservedWord(s string) bool {
	reserved := map[string]bool{
		"help": true, "version": true, "completion": true,
	}
	return reserved[s]
}

// parseManPage runs man and returns a Command with flags, subcommands, and positional args.
func parseManPage(program string) (*model.Command, error) {
	out, err := exec.Command("sh", "-c", "man "+program+" 2>/dev/null | col -bx").Output()
	if err != nil || len(out) == 0 {
		return nil, err
	}

	lines := strings.Split(string(out), "\n")
	cmd := &model.Command{Name: program}
	cmd.Flags = extractFlags(lines)
	for _, e := range extractSubcommands(lines, true) {
		cmd.Subcommands = append(cmd.Subcommands, &model.Command{
			Name:        e.name,
			Description: e.desc,
		})
	}
	cmd.Args = extractPositionalArgs(lines, program)
	return cmd, nil
}

// synopsisArgPattern matches positional arguments in SYNOPSIS lines.
// Matches both required <arg-name> and optional [<arg-name>] or [arg-name].
var synopsisArgPattern = regexp.MustCompile(`(\[?)<([a-zA-Z][a-zA-Z0-9_\- ]+)>(\]?)`)

// extractPositionalArgs parses the SYNOPSIS section to find positional arguments,
// then looks up their descriptions in the DESCRIPTION section.
func extractPositionalArgs(lines []string, program string) []model.Arg {
	// Step 1: find SYNOPSIS and collect the positional arg names in order.
	inSynopsis := false
	synopsisLines := []string{}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		upper := strings.ToUpper(trimmed)
		if upper == "SYNOPSIS" {
			inSynopsis = true
			continue
		}
		// Next all-uppercase section header ends synopsis
		if inSynopsis && trimmed != "" && upper == trimmed && len(trimmed) > 2 {
			break
		}
		if inSynopsis {
			synopsisLines = append(synopsisLines, line)
		}
	}

	if len(synopsisLines) == 0 {
		return nil
	}

	// Join wrapped synopsis lines into one string
	synopsis := strings.Join(synopsisLines, " ")

	// Remove the program name from the start
	if idx := strings.Index(synopsis, program); idx != -1 {
		synopsis = synopsis[idx+len(program):]
	}

	// Extract all positional arg tokens in order (skip flags -x / --xxx)
	type rawArg struct {
		name     string
		optional bool
	}
	var rawArgs []rawArg
	seen := map[string]bool{}

	matches := synopsisArgPattern.FindAllStringSubmatch(synopsis, -1)
	for _, m := range matches {
		// Normalize spaces/hyphens in name: "charset string" → "charset-string"
		rawName := strings.TrimSpace(m[2])
		name := strings.ToLower(strings.ReplaceAll(rawName, " ", "-"))
		optional := m[1] == "[" || m[3] == "]"
		if seen[name] {
			continue
		}
		// Skip common meta-placeholders that aren't real positional args
		if name == "args" || name == "options" || name == "command" || name == "cmd" {
			continue
		}
		seen[name] = true
		rawArgs = append(rawArgs, rawArg{name: name, optional: optional})
	}

	if len(rawArgs) == 0 {
		return nil
	}

	// Step 2: build a description map from the DESCRIPTION section.
	// Pattern: a line with just "arg-name" (possibly indented) followed by an indented description.
	descMap := extractArgDescriptions(lines, seen)

	// Step 3: assemble
	var args []model.Arg
	for _, r := range rawArgs {
		desc := descMap[r.name]
		if desc == "" {
			desc = r.name
		}
		args = append(args, model.Arg{
			Name:        r.name,
			Description: desc,
			Optional:    r.optional,
		})
	}
	return args
}

// extractArgDescriptions scans the DESCRIPTION section for entries of the form:
//
//	arg-name
//	       Description text here.
//
// Returns a map from lowercased name → first sentence of description.
func extractArgDescriptions(lines []string, names map[string]bool) map[string]string {
	result := map[string]string{}
	inDescription := false

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		upper := strings.ToUpper(trimmed)

		if upper == "DESCRIPTION" {
			inDescription = true
			continue
		}
		if inDescription && trimmed != "" && upper == trimmed && len(trimmed) > 2 {
			break // next section
		}
		if !inDescription {
			continue
		}

		// Look for a line that is just an arg name (indented 4-8 spaces, no special chars)
		lineIndent := indentOf(line)
		if lineIndent < 4 || lineIndent > 8 {
			continue
		}
		candidate := strings.ToLower(strings.ReplaceAll(trimmed, " ", "-"))
		if !names[candidate] && !names[strings.ReplaceAll(trimmed, " ", "-")] {
			continue
		}

		// Gather description from subsequent more-indented lines
		var descParts []string
		for j := i + 1; j < len(lines) && j < i+8; j++ {
			next := lines[j]
			nextTrimmed := strings.TrimSpace(next)
			if nextTrimmed == "" {
				break
			}
			if indentOf(next) <= lineIndent {
				break
			}
			descParts = append(descParts, nextTrimmed)
		}

		if len(descParts) > 0 {
			full := strings.Join(descParts, " ")
			// Collapse multiple spaces from man page formatting
			full = regexp.MustCompile(`\s{2,}`).ReplaceAllString(full, " ")
			// Trim to first sentence for brevity
			if idx := strings.Index(full, ".  "); idx != -1 {
				full = full[:idx+1]
			} else if idx := strings.Index(full, ". "); idx != -1 {
				full = full[:idx+1]
			}
			key := strings.ToLower(strings.ReplaceAll(trimmed, " ", "-"))
			if result[key] == "" {
				result[key] = full
			}
		}
	}
	return result
}

