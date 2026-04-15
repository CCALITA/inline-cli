package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/CCALITA/inline-cli/internal/config"
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
			line, err := reader.ReadString('\n')
			if err != nil {
				return fmt.Errorf("failed to read input: %w", err)
			}
			key := strings.TrimSpace(line)
			if key != "" {
				fmt.Println()
				fmt.Println("Add this to your shell profile:")
				fmt.Printf("  export ANTHROPIC_API_KEY=%s\n", key)
			}
		} else {
			fmt.Println()
			fmt.Println("\033[32m✓\033[0m ANTHROPIC_API_KEY is already set")
		}
	}

	if v := os.Getenv("INLINE_CLI_BACKEND"); v != "" && v != chosen.Name {
		fmt.Println()
		fmt.Printf("\033[33mWarning: INLINE_CLI_BACKEND=%s is set and will override this config.\033[0m\n", v)
		fmt.Println("Run: unset INLINE_CLI_BACKEND")
	}

	if err := config.SaveBackend(chosen.Name); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Println()
	fmt.Printf("\033[32m✓\033[0m Backend set to %q\n", chosen.Name)
	restartDaemonIfRunning()

	fmt.Println()
	fmt.Println("You're all set! Type something and press Ctrl+J (or Shift+Enter).")

	return nil
}
