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
	Use:   "ccs",
	Short: "Central Config Service CLI",
	Long:  "Command-line tool for managing schemas, tenants, and configuration values in the Central Config Service.",
}

func init() {
	pf := rootCmd.PersistentFlags()
	pf.StringVar(&flagServer, "server", envOrDefault("CCS_SERVER", "localhost:9090"), "gRPC server address")
	pf.StringVar(&flagSubject, "subject", envOrDefault("CCS_SUBJECT", ""), "actor identity (x-subject header)")
	pf.StringVar(&flagRole, "role", envOrDefault("CCS_ROLE", "superadmin"), "actor role (x-role header)")
	pf.StringVar(&flagTenantID, "tenant-id", envOrDefault("CCS_TENANT_ID", ""), "auth tenant ID (x-tenant-id header)")
	pf.StringVar(&flagToken, "token", envOrDefault("CCS_TOKEN", ""), "JWT bearer token")
	pf.StringVarP(&flagOutput, "output", "o", "table", "output format: table, json, yaml")
	pf.BoolVar(&flagInsecure, "insecure", envOrDefault("CCS_INSECURE", "true") == "true", "skip TLS verification")

	rootCmd.AddCommand(schemaCmd)
	rootCmd.AddCommand(tenantCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(watchCmd)
	rootCmd.AddCommand(lockCmd)
	rootCmd.AddCommand(auditCmd)
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
