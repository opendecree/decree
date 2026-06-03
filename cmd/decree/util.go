package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// mustGetString retrieves a string flag value from cmd. It panics if the flag
// does not exist (a programming error — the flag must be registered before use).
func mustGetString(cmd *cobra.Command, name string) string {
	v, err := cmd.Flags().GetString(name)
	if err != nil {
		panic(fmt.Sprintf("flag %q not registered: %v", name, err))
	}
	return v
}

// mustGetBool retrieves a bool flag value from cmd. It panics if the flag
// does not exist (a programming error — the flag must be registered before use).
func mustGetBool(cmd *cobra.Command, name string) bool {
	v, err := cmd.Flags().GetBool(name)
	if err != nil {
		panic(fmt.Sprintf("flag %q not registered: %v", name, err))
	}
	return v
}

// mustGetInt32 retrieves an int32 flag value from cmd. It panics if the flag
// does not exist (a programming error — the flag must be registered before use).
func mustGetInt32(cmd *cobra.Command, name string) int32 {
	v, err := cmd.Flags().GetInt32(name)
	if err != nil {
		panic(fmt.Sprintf("flag %q not registered: %v", name, err))
	}
	return v
}

// writeFileExclusive writes data to path using permission 0o600. If the file
// already exists and force is false, it returns an error. If force is true,
// the file is overwritten.
func writeFileExclusive(path string, data []byte, force bool) error {
	if !force {
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("file %q already exists; use --force to overwrite", path)
		}
	}
	return os.WriteFile(path, data, 0o600)
}
