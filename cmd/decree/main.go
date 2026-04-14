package main

import (
	"os"

	"github.com/spf13/cobra"
)

var (
	flagServer   string
	flagSubject  string
	flagRole     string
	flagTenantID string
	flagToken    string
	flagOutput   string
	flagInsecure bool
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "decree",
	Short: "OpenDecree CLI",
	Long:  "Command-line tool for managing schemas, tenants, and configuration values in OpenDecree.",
}

func init() {
	pf := rootCmd.PersistentFlags()
	pf.StringVar(&flagServer, "server", envOrDefault("DECREE_SERVER", "localhost:9090"), "gRPC server address")
	pf.StringVar(&flagSubject, "subject", envOrDefault("DECREE_SUBJECT", ""), "actor identity (x-subject header)")
	pf.StringVar(&flagRole, "role", envOrDefault("DECREE_ROLE", "superadmin"), "actor role (x-role header)")
	pf.StringVar(&flagTenantID, "tenant-id", envOrDefault("DECREE_TENANT_ID", ""), "auth tenant ID (x-tenant-id header)")
	pf.StringVar(&flagToken, "token", envOrDefault("DECREE_TOKEN", ""), "JWT bearer token")
	pf.StringVarP(&flagOutput, "output", "o", "table", "output format: table, json, yaml")
	pf.BoolVar(&flagInsecure, "insecure", envOrDefault("DECREE_INSECURE", "true") == "true", "skip TLS verification")

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
