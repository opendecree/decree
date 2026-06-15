// Command decree-docs generates documentation from decree schemas.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

// run executes the root command with the given arguments and returns the
// process exit code. Split from main so tests can drive the CLI in-process.
func run(args []string) int {
	ctx, stop := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
	)
	defer stop()

	rootCmd.SetArgs(args)
	if err := rootCmd.ExecuteContext(ctx); err != nil {
		_, _ = fmt.Fprintf(rootCmd.ErrOrStderr(), "Error: %s\n", err)
		return 1
	}
	return 0
}

var rootCmd = &cobra.Command{
	Use:   "decree-docs",
	Short: "Generate documentation from decree schemas",
	Long: `decree-docs is a standalone, multi-format documentation generator for
decree schemas. It loads schemas from local files or from a running decree
server and renders them as json, md, mdx, or html, with built-in themes,
CSS style injection, and a template override system.

This build loads schemas from local YAML files and emits the JSON
documentation model; the md/mdx/html backends and server mode land in
upcoming releases.`,
	SilenceUsage:  true,
	SilenceErrors: true,
}
