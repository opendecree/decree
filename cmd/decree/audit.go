package main

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/opendecree/decree/sdk/adminclient"
)

var auditCmd = &cobra.Command{
	Use:   "audit",
	Short: "Query audit logs and usage statistics",
	Long:  "Query the configuration change history and field read usage statistics. Useful for compliance auditing and identifying unused fields.",
}

var auditQueryCmd = &cobra.Command{
	Use:   "query",
	Short: "Query the config change audit log",
	RunE: func(cmd *cobra.Command, args []string) error {
		conn, err := dialServer()
		if err != nil {
			return err
		}
		defer func() { _ = conn.Close() }()

		var filters []adminclient.AuditFilter
		if v, _ := cmd.Flags().GetString("tenant"); v != "" {
			filters = append(filters, adminclient.WithAuditTenant(v))
		}
		if v, _ := cmd.Flags().GetString("actor"); v != "" {
			filters = append(filters, adminclient.WithAuditActor(v))
		}
		if v, _ := cmd.Flags().GetString("field"); v != "" {
			filters = append(filters, adminclient.WithAuditField(v))
		}
		if v, _ := cmd.Flags().GetString("since"); v != "" {
			d, err := time.ParseDuration(v)
			if err != nil {
				return fmt.Errorf("invalid --since duration: %w", err)
			}
			t := time.Now().Add(-d)
			filters = append(filters, adminclient.WithAuditTimeRange(&t, nil))
		}

		entries, err := newAdminClient(conn).QueryWriteLog(cmd.Context(), filters...)
		if err != nil {
			return err
		}
		rows := tableRows([]string{"TIME", "ACTOR", "ACTION", "FIELD", "OLD", "NEW"})
		for _, e := range entries {
			rows = append(rows, []string{
				e.CreatedAt.Format("2006-01-02 15:04:05"),
				e.Actor, e.Action, e.FieldPath, e.OldValue, e.NewValue,
			})
		}
		return printOutput(rows)
	},
}

var auditUsageCmd = &cobra.Command{
	Use:   "usage <tenant-id> <field-path>",
	Short: "Show read usage statistics for a field",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		conn, err := dialServer()
		if err != nil {
			return err
		}
		defer func() { _ = conn.Close() }()

		stats, err := newAdminClient(conn).GetFieldUsage(cmd.Context(), args[0], args[1], nil, nil)
		if err != nil {
			return err
		}
		return printOutput(tableRows(
			[]string{"FIELD", "READ_COUNT", "LAST_READ_BY"},
			[]string{stats.FieldPath, fmt.Sprintf("%d", stats.ReadCount), stats.LastReadBy},
		))
	},
}

var auditVerifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Verify the tamper-evident audit chain for a tenant",
	Long: `Fetches all audit entries for the tenant (or the global chain if --tenant is
omitted), recomputes each entry_hash, and reports any breaks.

Requires migration 002_audit_tamper_evident to be applied.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		conn, err := dialServer()
		if err != nil {
			return err
		}
		defer func() { _ = conn.Close() }()

		tenantID, _ := cmd.Flags().GetString("tenant")
		result, err := newAdminClient(conn).VerifyChain(cmd.Context(), tenantID)
		if err != nil {
			return err
		}

		if result.OK {
			fmt.Printf("OK — %d entries, chain intact\n", result.Total)
			return nil
		}

		fmt.Printf("FAIL — %d breaks in %d entries\n", len(result.Breaks), result.Total)
		rows := tableRows([]string{"POSITION", "ENTRY_ID", "GOT", "WANT"})
		for _, b := range result.Breaks {
			got := b.Got
			want := b.Want
			if len(got) > 12 {
				got = got[:12] + "…"
			}
			if len(want) > 12 {
				want = want[:12] + "…"
			}
			rows = append(rows, []string{
				fmt.Sprintf("%d", b.Position),
				b.EntryID,
				got,
				want,
			})
		}
		return printOutput(rows)
	},
}

var auditUnusedCmd = &cobra.Command{
	Use:   "unused <tenant-id> <since-duration>",
	Short: "Find fields not read since the given duration (e.g. 7d, 24h)",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		d, err := parseDuration(args[1])
		if err != nil {
			return fmt.Errorf("invalid duration: %w", err)
		}
		since := time.Now().Add(-d)

		conn, err := dialServer()
		if err != nil {
			return err
		}
		defer func() { _ = conn.Close() }()

		fields, err := newAdminClient(conn).GetUnusedFields(cmd.Context(), args[0], since)
		if err != nil {
			return err
		}
		if len(fields) == 0 {
			fmt.Println("No unused fields.")
			return nil
		}
		rows := tableRows([]string{"UNUSED_FIELD"})
		for _, f := range fields {
			rows = append(rows, []string{f})
		}
		return printOutput(rows)
	},
}

// parseDuration extends time.ParseDuration with day support (e.g. "7d").
func parseDuration(s string) (time.Duration, error) {
	if len(s) > 1 && s[len(s)-1] == 'd' {
		var n float64
		if _, err := fmt.Sscanf(s[:len(s)-1], "%f", &n); err == nil {
			return time.Duration(n * float64(24*time.Hour)), nil
		}
	}
	return time.ParseDuration(s)
}

func init() {
	auditQueryCmd.Flags().String("tenant", "", "filter by tenant ID")
	auditQueryCmd.Flags().String("actor", "", "filter by actor")
	auditQueryCmd.Flags().String("field", "", "filter by field path")
	auditQueryCmd.Flags().String("since", "", "show entries from the last duration (e.g. 24h, 7d)")

	auditVerifyCmd.Flags().String("tenant", "", "tenant ID to verify (empty = global chain)")

	auditCmd.AddCommand(auditQueryCmd)
	auditCmd.AddCommand(auditUsageCmd)
	auditCmd.AddCommand(auditUnusedCmd)
	auditCmd.AddCommand(auditVerifyCmd)
}
