package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
	"github.com/TerenceU/the-autocompletor/internal/ai"
	"github.com/TerenceU/the-autocompletor/internal/generator"
	"github.com/TerenceU/the-autocompletor/internal/installer"
	"github.com/TerenceU/the-autocompletor/internal/model"
	"github.com/TerenceU/the-autocompletor/internal/parser"
	"github.com/TerenceU/the-autocompletor/internal/shell"
)

var (
	flagShell  string
	flagInstall bool
	flagAI     string
	flagAPIKey string
	flagModel  string
)

var rootCmd = &cobra.Command{
	Use:   "theautocompletor <program>",
	Short: "Generate shell completions for any CLI program",
	Long: `theautocompletor generates shell completion scripts by analyzing a program's
man page, --help output, and subcommands recursively.

If the program cannot be analyzed, an AI fallback (Ollama or OpenAI) can be used.

If you want autocompletions for this program try:
  theautocompletor theautocompletor --install 

Examples:
  theautocompletor gobuster
  theautocompletor gobuster --install
  theautocompletor gobuster --shell fish --install
  theautocompletor gobuster --ai ollama
  theautocompletor gobuster --ai openai --api-key sk-...`,
	Args:              cobra.ExactArgs(1),
	RunE:              run,
	SilenceErrors:     true,
}

func init() {
	rootCmd.Flags().StringVar(&flagShell, "shell", "", "Target shell: fish, bash, zsh (auto-detected if not set)")
	rootCmd.Flags().BoolVar(&flagInstall, "install", false, "Install completions to the shell's completions directory")
	rootCmd.Flags().StringVar(&flagAI, "ai", "", "AI fallback to use: ollama, openai")
	rootCmd.Flags().StringVar(&flagAPIKey, "api-key", "", "API key for OpenAI (or set OPENAI_API_KEY env var)")
	rootCmd.Flags().StringVar(&flagModel, "model", "", "AI model to use (default: llama3 for ollama, gpt-4o-mini for openai)")
}

func run(cmd *cobra.Command, args []string) error {
	program := args[0]

	// Resolve target shell
	var sh shell.Shell
	var err error
	if flagShell != "" {
		sh, err = shell.Parse(flagShell)
	} else {
		sh, err = shell.Detect()
	}
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "→ Generating %s completions for %q\n", sh, program)

	// Build command tree
	cmdTree, parseErr := parser.Parse(program)
	if parseErr != nil || (len(cmdTree.Flags) == 0 && len(cmdTree.Subcommands) == 0) {
		if flagAI == "" {
			return fmt.Errorf(
				"could not extract completions for %q (no man page or --help output found)\n"+
					"Tip: use --ai ollama or --ai openai to use AI as fallback", program,
			)
		}
		fmt.Fprintf(os.Stderr, "→ No completions found via help/man, falling back to AI (%s)\n", flagAI)
		cmdTree, err = runAI(program, sh)
		if err != nil {
			return fmt.Errorf("AI fallback failed: %w", err)
		}
	}

	// Generate completions
	var output string
	switch sh {
	case shell.Fish:
		output = generator.Fish(cmdTree)
	case shell.Bash:
		output = generator.Bash(cmdTree)
	case shell.Zsh:
		output = generator.Zsh(cmdTree)
	}

	// Install or print
	if flagInstall {
		path, err := installer.Install(sh, program, output)
		if err != nil {
			return fmt.Errorf("install failed: %w", err)
		}
		fmt.Fprintf(os.Stderr, "✓ Completions installed to %s\n", path)
	} else {
		fmt.Print(output)
	}

	return nil
}

func runAI(program string, sh shell.Shell) (*model.Command, error) {
	switch flagAI {
	case "ollama":
		return ai.Ollama(program, sh, ai.OllamaOptions{Model: flagModel})
	case "openai":
		return ai.OpenAI(program, sh, ai.OpenAIOptions{APIKey: flagAPIKey, Model: flagModel})
	default:
		return nil, fmt.Errorf("unknown AI backend %q (use ollama or openai)", flagAI)
	}
}

func main() {
	// Register "tac" alias only if the system tac command is not present
	if _, err := exec.LookPath("tac"); err != nil {
		os.Args[0] = "tac" // cosmetic only; cobra uses Use field
	}

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
