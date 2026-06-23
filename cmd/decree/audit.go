package main

import (
	"fmt"
	"io"
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
		if v := mustGetString(cmd, "tenant"); v != "" {
			filters = append(filters, adminclient.WithAuditTenant(v))
		}
		if v := mustGetString(cmd, "actor"); v != "" {
			filters = append(filters, adminclient.WithAuditActor(v))
		}
		if v := mustGetString(cmd, "field"); v != "" {
			filters = append(filters, adminclient.WithAuditField(v))
		}
		if v := mustGetString(cmd, "since"); v != "" {
			d, err := parseDuration(v)
			if err != nil {
				return fmt.Errorf("invalid --since duration: %w", err)
			}
			t := time.Now().Add(-d)
			filters = append(filters, adminclient.WithAuditTimeRange(&t, nil))
		}

		admin, err := newAdminClient(conn)
		if err != nil {
			return err
		}
		entries, err := admin.QueryWriteLog(cmd.Context(), filters...)
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

		admin, err := newAdminClient(conn)
		if err != nil {
			return err
		}
		stats, err := admin.GetFieldUsage(cmd.Context(), args[0], args[1], nil, nil)
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

Requires the server's database schema to be up to date so that the tamper-evident
hash columns are populated.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		conn, err := dialServer()
		if err != nil {
			return err
		}
		defer func() { _ = conn.Close() }()

		tenantID := mustGetString(cmd, "tenant")
		admin, err := newAdminClient(conn)
		if err != nil {
			return err
		}
		result, err := admin.VerifyChain(cmd.Context(), tenantID)
		if err != nil {
			return err
		}

		return printVerifyResult(cmd.OutOrStdout(), result)
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

		admin, err := newAdminClient(conn)
		if err != nil {
			return err
		}
		fields, err := admin.GetUnusedFields(cmd.Context(), args[0], since)
		if err != nil {
			return err
		}
		if len(fields) == 0 {
			printStatus(cmd, "No unused fields.\n")
			return nil
		}
		rows := tableRows([]string{"UNUSED_FIELD"})
		for _, f := range fields {
			rows = append(rows, []string{f})
		}
		return printOutput(rows)
	},
}

// printVerifyResult writes the chain-verification outcome to w and returns a
// non-nil error when breaks are found so that the CLI exits non-zero.
func printVerifyResult(w io.Writer, result adminclient.VerifyChainResult) error {
	if result.OK {
		_, _ = fmt.Fprintf(w, "OK — %d entries, chain intact\n", result.Total)
		return nil
	}

	_, _ = fmt.Fprintf(w, "FAIL — %d breaks in %d entries\n", len(result.Breaks), result.Total)
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
	if err := printTable(w, rows); err != nil {
		return err
	}
	return fmt.Errorf("audit chain broken: %d break(s) found", len(result.Breaks))
}

// parseDuration extends time.ParseDuration with day support (e.g. "7d").
// It rejects inputs like "7dd" or "7d1h" that look day-like but have trailing
// characters — callers must use standard Go duration syntax for those.
func parseDuration(s string) (time.Duration, error) {
	if len(s) > 1 && s[len(s)-1] == 'd' {
		prefix := s[:len(s)-1]
		var n float64
		var rest string
		if _, err := fmt.Sscanf(prefix, "%f%s", &n, &rest); err == nil {
			// Sscanf consumed a number but left trailing characters — reject.
			return 0, fmt.Errorf("invalid duration %q: unexpected characters after day count", s)
		}
		// Only a number before 'd' — valid.
		if _, err := fmt.Sscanf(prefix, "%f", &n); err == nil {
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
