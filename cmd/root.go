package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "AIterate",
	Short: "AIterate - AI-powered code generation with test-driven development",
	Long: `AIterate is a tool that uses AI to generate and iterate on code until it passes tests.
It first generates tests based on your requirements, then creates an implementation,
and iteratively improves the code until all tests pass.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
