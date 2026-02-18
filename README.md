# theautocompleter

Generate shell completion scripts for any CLI program — automatically.

## How it works

1. Tries the **man page** first
2. Falls back to `--help` output
3. Recursively discovers **subcommands** and their flags
4. If nothing is found, uses an **AI fallback** (Ollama or OpenAI)

## Usage

```bash
theautocompleter <program> [flags]
```

| Example | Description |
|---------|-------------|
| `theautocompleter gobuster` | Auto-detect shell, print to stdout |
| `theautocompleter gobuster --install` | Auto-detect shell, install to shell dir |
| `theautocompleter gobuster --shell fish` | Force fish output |
| `theautocompleter gobuster --shell fish --install` | Force fish and install |
| `theautocompleter gobuster --ai ollama` | Use local Ollama as fallback |
| `theautocompleter gobuster --ai openai --api-key sk-...` | Use OpenAI as fallback |

> **Alias `tac`**: if the system `tac` command is not present, you can also use `tac <program>` as a shorter alias.

## Supported shells

| Shell | Install directory |
|-------|-------------------|
| Fish  | `~/.config/fish/completions/` |
| Bash  | `~/.bash_completion.d/` |
| Zsh   | `~/.zsh/completions/` |

## Installation

```bash
git clone https://github.com/terencetachiona/the-autocompletor
cd the-autocompletor
make install
```

## Flags

| Flag | Description |
|------|-------------|
| `--shell` | Target shell: `fish`, `bash`, `zsh` (auto-detected if not set) |
| `--install` | Install completions to the shell's directory instead of stdout |
| `--ai` | AI fallback: `ollama` or `openai` |
| `--api-key` | OpenAI API key (or set `OPENAI_API_KEY` env var) |
| `--model` | AI model override |
