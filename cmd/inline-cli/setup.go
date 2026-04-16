package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/CCALITA/inline-cli/internal/config"
	"github.com/CCALITA/inline-cli/internal/render"
)

func newSetupCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "setup",
		Short: "Interactive first-time setup",
		RunE:  runSetup,
	}
}

func runSetup(cmd *cobra.Command, args []string) error {
	fmt.Println("Welcome to inline-cli!")
	fmt.Println()
	fmt.Println("Select a backend:")
	fmt.Println()

	for i, b := range backends {
		fmt.Printf("  %d) %-10s — %s%s\n", i+1, b.Name, b.Desc, b.installStatus())
	}

	fmt.Println()

	reader := bufio.NewReader(os.Stdin)
	var chosen backendInfo

	for {
		fmt.Printf("Enter choice [1-%d]: ", len(backends))
		line, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read input: %w", err)
		}
		line = strings.TrimSpace(line)
		n, err := strconv.Atoi(line)
		if err != nil || n < 1 || n > len(backends) {
			fmt.Println("Invalid choice. Please enter a number.")
			continue
		}
		chosen = backends[n-1]
		break
	}

	if chosen.Name == "api" {
		existing := os.Getenv("ANTHROPIC_API_KEY")
		if existing == "" {
			fmt.Println()
			fmt.Print("Enter your Anthropic API key (or press Enter to skip): ")
			keyBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
			fmt.Println() // newline after hidden input
			if err != nil {
				return fmt.Errorf("failed to read input: %w", err)
			}
			key := strings.TrimSpace(string(keyBytes))
			if key != "" {
				fmt.Println("Add this to your shell profile:")
				fmt.Printf("  export ANTHROPIC_API_KEY='%s'\n", strings.ReplaceAll(key, "'", "'\\''"))
			}
		} else {
			fmt.Println()
			fmt.Printf("%s ANTHROPIC_API_KEY is already set\n", render.Green("✓"))
		}
	}

	if err := config.SaveBackend(chosen.Name); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Println()
	fmt.Printf("%s Backend set to %q\n", render.Green("✓"), chosen.Name)
	restartDaemonIfRunning()

	fmt.Println()
	fmt.Println("You're all set! Type something and press Ctrl+J (or Shift+Enter).")

	return nil
}
