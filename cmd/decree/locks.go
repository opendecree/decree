package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var lockCmd = &cobra.Command{
	Use:   "lock",
	Short: "Manage field locks",
	Long:  "Lock and unlock configuration fields. Locked fields cannot be modified by admin or user roles — only superadmins can bypass locks.",
}

var lockSetCmd = &cobra.Command{
	Use:   "set <tenant-id> <field-path>",
	Short: "Lock a field (prevents modification by non-superadmin)",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		conn, err := dialServer()
		if err != nil {
			return err
		}
		defer func() { _ = conn.Close() }()

		if err := newAdminClient(conn).LockField(cmd.Context(), args[0], args[1]); err != nil {
			return err
		}
		fmt.Printf("Locked %s\n", args[1])
		return nil
	},
}

var lockRemoveCmd = &cobra.Command{
	Use:   "remove <tenant-id> <field-path>",
	Short: "Unlock a field",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		conn, err := dialServer()
		if err != nil {
			return err
		}
		defer func() { _ = conn.Close() }()

		if err := newAdminClient(conn).UnlockField(cmd.Context(), args[0], args[1]); err != nil {
			return err
		}
		fmt.Printf("Unlocked %s\n", args[1])
		return nil
	},
}

var lockListCmd = &cobra.Command{
	Use:   "list <tenant-id>",
	Short: "List field locks for a tenant",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		conn, err := dialServer()
		if err != nil {
			return err
		}
		defer func() { _ = conn.Close() }()

		locks, err := newAdminClient(conn).ListFieldLocks(cmd.Context(), args[0])
		if err != nil {
			return err
		}
		if len(locks) == 0 {
			fmt.Println("No locks.")
			return nil
		}
		rows := tableRows([]string{"FIELD_PATH", "LOCKED_VALUES"})
		for _, l := range locks {
			vals := "-"
			if len(l.LockedValues) > 0 {
				vals = fmt.Sprintf("%v", l.LockedValues)
			}
			rows = append(rows, []string{l.FieldPath, vals})
		}
		return printOutput(rows)
	},
}

func init() {
	lockCmd.AddCommand(lockSetCmd)
	lockCmd.AddCommand(lockRemoveCmd)
	lockCmd.AddCommand(lockListCmd)
}
