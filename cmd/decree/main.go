package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

var (
	flagServer      string
	flagSubject     string
	flagRole        string
	flagTenantID    string
	flagToken       string
	flagTokenFile   string
	flagOutput      string
	flagInsecure    bool
	flagWait        bool
	flagWaitTimeout string
)

func main() {
	ctx, stop := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
	)
	defer stop()

	if err := rootCmd.ExecuteContext(ctx); err != nil {
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:           "decree",
	Short:         "OpenDecree CLI",
	Long:          "Command-line tool for managing schemas, tenants, and configuration values in OpenDecree.",
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
		if !flagWait || isOfflineInvocation(cmd) {
			return nil
		}
		timeout, err := time.ParseDuration(flagWaitTimeout)
		if err != nil {
			return fmt.Errorf("invalid --wait-timeout: %w", err)
		}
		conn, err := dialServer()
		if err != nil {
			return err
		}
		defer conn.Close()
		fmt.Fprintf(cmd.ErrOrStderr(), "Waiting for server %s (timeout %s)...\n", flagServer, timeout)
		if err := waitForServer(cmd.Context(), conn, timeout); err != nil {
			return err
		}
		fmt.Fprintf(cmd.ErrOrStderr(), "Server ready.\n")
		return nil
	},
}

// isOfflineInvocation returns true when the given command does not need a
// server connection and therefore should not block on --wait.
func isOfflineInvocation(cmd *cobra.Command) bool {
	switch cmd.Name() {
	case "validate", "version", "gen-docs", "gen-man":
		return true
	case "diff":
		old, _ := cmd.Flags().GetString("old")
		new, _ := cmd.Flags().GetString("new")
		return old != "" && new != ""
	case "docgen":
		file, _ := cmd.Flags().GetString("file")
		return file != ""
	}
	return false
}

func init() {
	pf := rootCmd.PersistentFlags()
	pf.StringVar(&flagServer, "server", envOrDefault("DECREE_SERVER", "localhost:9090"), "gRPC server address")
	pf.StringVar(&flagSubject, "subject", envOrDefault("DECREE_SUBJECT", ""), "actor identity (x-subject header)")
	pf.StringVar(&flagRole, "role", envOrDefault("DECREE_ROLE", "superadmin"), "actor role (x-role header)")
	pf.StringVar(&flagTenantID, "tenant-id", envOrDefault("DECREE_TENANT_ID", ""), "auth tenant ID (x-tenant-id header)")
	pf.StringVar(&flagToken, "token", envOrDefault("DECREE_TOKEN", ""), "JWT bearer token (prefer DECREE_TOKEN env var to avoid shell history exposure)")
	pf.StringVar(&flagTokenFile, "token-file", "", "path to a file containing the JWT bearer token (takes precedence over --token)")
	pf.StringVarP(&flagOutput, "output", "o", "table", "output format: table, json, yaml")
	pf.BoolVar(&flagInsecure, "insecure", envOrDefault("DECREE_INSECURE", "false") == "true", "disable TLS (plaintext); for local development only")
	pf.BoolVar(&flagWait, "wait", false, "wait for the server to be ready before executing the command")
	pf.StringVar(&flagWaitTimeout, "wait-timeout", envOrDefault("DECREE_WAIT_TIMEOUT", "60s"), "maximum time to wait for server readiness")

	// Flag completions.
	_ = rootCmd.RegisterFlagCompletionFunc("output", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return []string{"table", "json", "yaml"}, cobra.ShellCompDirectiveNoFileComp
	})
	_ = rootCmd.RegisterFlagCompletionFunc("role", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return []string{"superadmin", "admin", "user"}, cobra.ShellCompDirectiveNoFileComp
	})

	rootCmd.AddCommand(schemaCmd)
	rootCmd.AddCommand(tenantCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(watchCmd)
	rootCmd.AddCommand(lockCmd)
	rootCmd.AddCommand(auditCmd)
	rootCmd.AddCommand(serverCmd)
	rootCmd.AddCommand(diffCmd)
	rootCmd.AddCommand(docgenCmd)
	rootCmd.AddCommand(validateCmd)
	rootCmd.AddCommand(seedCmd)
	rootCmd.AddCommand(dumpCmd)
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
