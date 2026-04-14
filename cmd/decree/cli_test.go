package main

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Command structure ---

func TestRootCommand_HasAllSubcommands(t *testing.T) {
	expected := []string{"schema", "tenant", "config", "watch", "lock", "audit", "diff", "docgen", "validate", "seed", "dump"}
	names := make([]string, 0, len(expected))
	for _, cmd := range rootCmd.Commands() {
		names = append(names, cmd.Name())
	}
	for _, exp := range expected {
		assert.Contains(t, names, exp, "missing subcommand: %s", exp)
	}
}

func TestSchemaCommand_HasSubcommands(t *testing.T) {
	expected := []string{"create", "get", "list", "publish", "delete", "export", "import"}
	names := make([]string, 0, len(expected))
	for _, cmd := range schemaCmd.Commands() {
		names = append(names, cmd.Name())
	}
	for _, exp := range expected {
		assert.Contains(t, names, exp, "missing schema subcommand: %s", exp)
	}
}

func TestConfigCommand_HasSubcommands(t *testing.T) {
	expected := []string{"get", "get-all", "set", "set-many", "versions", "rollback", "export", "import"}
	names := make([]string, 0, len(expected))
	for _, cmd := range configCmd.Commands() {
		names = append(names, cmd.Name())
	}
	for _, exp := range expected {
		assert.Contains(t, names, exp, "missing config subcommand: %s", exp)
	}
}

func TestTenantCommand_HasSubcommands(t *testing.T) {
	expected := []string{"create", "get", "list", "delete"}
	names := make([]string, 0, len(expected))
	for _, cmd := range tenantCmd.Commands() {
		names = append(names, cmd.Name())
	}
	for _, exp := range expected {
		assert.Contains(t, names, exp, "missing tenant subcommand: %s", exp)
	}
}

func TestLockCommand_HasSubcommands(t *testing.T) {
	expected := []string{"set", "remove", "list"}
	names := make([]string, 0, len(expected))
	for _, cmd := range lockCmd.Commands() {
		names = append(names, cmd.Name())
	}
	for _, exp := range expected {
		assert.Contains(t, names, exp, "missing lock subcommand: %s", exp)
	}
}

func TestAuditCommand_HasSubcommands(t *testing.T) {
	expected := []string{"query", "usage", "unused"}
	names := make([]string, 0, len(expected))
	for _, cmd := range auditCmd.Commands() {
		names = append(names, cmd.Name())
	}
	for _, exp := range expected {
		assert.Contains(t, names, exp, "missing audit subcommand: %s", exp)
	}
}

// --- Argument validation ---

func TestSchemaGet_RequiresSchemaID(t *testing.T) {
	rootCmd.SetArgs([]string{"schema", "get"})
	err := rootCmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "accepts 1 arg")
}

func TestConfigGet_RequiresTenantAndField(t *testing.T) {
	rootCmd.SetArgs([]string{"config", "get", "only-one-arg"})
	err := rootCmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "accepts 2 arg")
}

func TestConfigSet_RequiresThreeArgs(t *testing.T) {
	rootCmd.SetArgs([]string{"config", "set", "tenant", "field"})
	err := rootCmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "accepts 3 arg")
}

func TestWatch_RequiresTenantID(t *testing.T) {
	rootCmd.SetArgs([]string{"watch"})
	err := rootCmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "requires at least 1 arg")
}

// --- Output formatting ---

func TestPrintTable(t *testing.T) {
	var buf bytes.Buffer
	rows := tableRows(
		[]string{"NAME", "VERSION"},
		[]string{"payments", "3"},
		[]string{"settlement", "1"},
	)
	err := printTable(&buf, rows)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "NAME")
	assert.Contains(t, output, "payments")
	assert.Contains(t, output, "settlement")
	// Should have separator line.
	lines := strings.Split(output, "\n")
	assert.GreaterOrEqual(t, len(lines), 4) // header + sep + 2 rows
}

func TestPrintJSON(t *testing.T) {
	var buf bytes.Buffer
	data := map[string]string{"key": "value"}
	err := printJSON(&buf, data)
	require.NoError(t, err)
	assert.Contains(t, buf.String(), `"key": "value"`)
}

func TestPrintYAML(t *testing.T) {
	var buf bytes.Buffer
	data := map[string]string{"key": "value"}
	err := printYAML(&buf, data)
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "key: value")
}

func TestTableRows_Empty(t *testing.T) {
	rows := tableRows([]string{"A", "B"})
	assert.Len(t, rows, 1) // just headers
}

// --- Helpers ---

func TestParseDuration_Standard(t *testing.T) {
	d, err := parseDuration("24h")
	require.NoError(t, err)
	assert.Equal(t, 24*time.Hour, d)
}

func TestParseDuration_Days(t *testing.T) {
	d, err := parseDuration("7d")
	require.NoError(t, err)
	assert.Equal(t, 7*24*time.Hour, d)
}

func TestParseDuration_Invalid(t *testing.T) {
	_, err := parseDuration("abc")
	assert.Error(t, err)
}

// --- Completions ---

func TestCompletionScripts(t *testing.T) {
	for _, shell := range []string{"bash", "zsh", "fish", "powershell"} {
		t.Run(shell, func(t *testing.T) {
			var buf bytes.Buffer
			var err error
			switch shell {
			case "bash":
				err = rootCmd.GenBashCompletionV2(&buf, true)
			case "zsh":
				err = rootCmd.GenZshCompletion(&buf)
			case "fish":
				err = rootCmd.GenFishCompletion(&buf, true)
			case "powershell":
				err = rootCmd.GenPowerShellCompletion(&buf)
			}
			require.NoError(t, err)
			assert.NotEmpty(t, buf.String())
		})
	}
}
