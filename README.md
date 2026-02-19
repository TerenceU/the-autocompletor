# theautocompletor

Generate shell completion scripts for any CLI program — automatically.

> 🤖 **This project was built with AI assistance (GitHub Copilot).** It's an experiment in AI-driven development — the code, architecture, and docs were all shaped through a conversation between a human and an AI pair programmer.
>
> **Contributions of any kind are very welcome** — bug reports, new shell support, better parsing heuristics, docs, tests, anything. See [CONTRIBUTING.md](CONTRIBUTING.md) to get started.

## How it works

1. Tries the **man page** first
2. Falls back to `--help` output
3. Recursively discovers **subcommands** and their flags
4. If nothing is found, uses an **AI fallback** (Ollama or OpenAI)

## Usage

```bash
theautocompletor <program> [flags]
```

| Example | Description |
|---------|-------------|
| `theautocompletor gobuster` | Auto-detect shell, print to stdout |
| `theautocompletor gobuster --install` | Auto-detect shell, install to shell dir |
| `theautocompletor gobuster --shell fish` | Force fish output |
| `theautocompletor gobuster --shell fish --install` | Force fish and install |
| `theautocompletor gobuster --ai ollama` | Use local Ollama as fallback |
| `theautocompletor gobuster --ai openai --api-key sk-...` | Use OpenAI as fallback |

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

## Support

If you find this useful, consider buying me a coffee ☕

[![Ko-fi](https://ko-fi.com/img/githubbutton_sm.svg)](https://ko-fi.com/terenceusai)
