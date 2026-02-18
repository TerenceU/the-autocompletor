# Contributing to theautocompletor

First off — thanks for being here! This project was born out of a conversation between a human and an AI (GitHub Copilot), and it's very much a work in progress. Contributions of any kind are genuinely welcome: bug reports, new features, better parsing, more shell support, tests, docs, whatever you can bring.

---

## Project overview

`theautocompletor` generates shell completion scripts for any CLI program by reading its man page and `--help` output recursively. It has an optional AI fallback (Ollama / OpenAI) for programs that don't document themselves well.

### Directory structure

```
the-autocompletor/
├── main.go                     # CLI entry point (cobra)
├── internal/
│   ├── model/
│   │   └── model.go            # Shared structs: Flag, Command
│   ├── parser/
│   │   ├── parser.go           # Orchestrator: man → --help → recursive subcommands
│   │   └── help.go             # Regex-based flag + subcommand extractor
│   ├── generator/
│   │   ├── fish.go             # Fish completion format
│   │   ├── bash.go             # Bash completion format
│   │   └── zsh.go              # Zsh completion format
│   ├── shell/
│   │   └── detect.go           # Auto-detect current shell from env vars
│   ├── installer/
│   │   └── installer.go        # Write completions to the correct shell directory
│   └── ai/
│       ├── ollama.go           # Ollama local AI fallback
│       └── openai.go           # OpenAI API fallback
├── Makefile
└── README.md
```

### How parsing works

1. `parser.Parse(program)` tries `man <program> | col -bx` first
2. If the man page yields flags, it also calls `--help` to discover subcommands (merged in)
3. Otherwise falls back to `--help` / `-h` output
4. For each discovered subcommand, it recurses (`maxDepth = 3`) calling `<program> <sub> --help`
5. Pager programs (less, man) are suppressed via env vars: `PAGER=cat`, `GIT_PAGER=cat`, `MANPAGER=cat`, `TERM=dumb`

The core parsing logic is in `internal/parser/help.go`:
- `extractFlags(lines []string)` — two-pass: first joins multi-line flag definitions (man page style has flag on one line, description on the next), then applies `splitLinePattern` to separate flags from descriptions
- `extractSubcommands(lines []string)` — finds subcommand names + descriptions, handles both explicit `COMMANDS:` headers and git-style unlabelled lists

---

## Known limitations / good first issues

- **No tests yet** — the highest value contribution right now. Even a table-driven test over `extractFlags` with sample `--help` snippets would be great.
- **Subcommand descriptions missing for man-page-first programs** — when a program has a man page, subcommand flags come from `<program> <sub> --help` but the top-level subcommand description is only populated if `parseHelpRecursive` returns one. Some descriptions end up empty.
- **Zsh and Bash generators are minimal** — they work but lack context-awareness (subcommand-conditional completions). Compare to `fish.go` for reference.
- **False-positive subcommands** — `extractSubcommands` uses a heuristic (`^\s{2,4}word  description`) that can pick up non-subcommand lines from some programs.
- **No support for programs that use `help <subcommand>` instead of `<program> <subcommand> --help`** — e.g. some custom CLIs.
- **AI fallback is untested against real Ollama/OpenAI responses** — the prompt is simple and the response parsing is naive.

---

## Getting started

```bash
git clone https://github.com/terencetachiona/the-autocompletor
cd the-autocompletor
go build -o theautocompletor .

# Try it
./theautocompletor gobuster --shell fish
./theautocompletor git --shell fish
```

Requirements: Go 1.21+

---

## Submitting changes

- Open an issue first if you're planning something big, so we can discuss
- PRs are welcome for any size change — no contribution is too small
- No strict style rules; just try to match the existing code style
- If you add a new shell generator, add it to the `switch` in `main.go` and document it in `README.md`

---

## Ideas for future work

- `--update` flag: re-generate completions for all previously installed programs
- A `--verbose` flag for debugging parse output
- Support for programs that use positional arguments with fixed values (e.g. `systemctl start <unit>`)
- PowerShell generator
- Homebrew formula / AUR package
- A small test suite with `--help` fixtures for popular programs
