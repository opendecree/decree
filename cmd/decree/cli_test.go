package main

import (
	"bytes"
	"slices"
	"strings"
	"testing"
	"time"
)

// --- Command structure ---

func TestRootCommand_HasAllSubcommands(t *testing.T) {
	expected := []string{"schema", "tenant", "config", "watch", "lock", "audit", "diff", "docgen", "validate", "seed", "dump"}
	names := make([]string, 0, len(expected))
	for _, cmd := range rootCmd.Commands() {
		names = append(names, cmd.Name())
	}
	for _, exp := range expected {
		if !slices.Contains(names, exp) {
			t.Errorf("missing subcommand: %s", exp)
		}
	}
}

func TestSchemaCommand_HasSubcommands(t *testing.T) {
	expected := []string{"create", "get", "list", "publish", "delete", "export", "import"}
	names := make([]string, 0, len(expected))
	for _, cmd := range schemaCmd.Commands() {
		names = append(names, cmd.Name())
	}
	for _, exp := range expected {
		if !slices.Contains(names, exp) {
			t.Errorf("missing schema subcommand: %s", exp)
		}
	}
}

func TestConfigCommand_HasSubcommands(t *testing.T) {
	expected := []string{"get", "get-all", "set", "set-many", "versions", "rollback", "export", "import"}
	names := make([]string, 0, len(expected))
	for _, cmd := range configCmd.Commands() {
		names = append(names, cmd.Name())
	}
	for _, exp := range expected {
		if !slices.Contains(names, exp) {
			t.Errorf("missing config subcommand: %s", exp)
		}
	}
}

func TestTenantCommand_HasSubcommands(t *testing.T) {
	expected := []string{"create", "get", "list", "delete"}
	names := make([]string, 0, len(expected))
	for _, cmd := range tenantCmd.Commands() {
		names = append(names, cmd.Name())
	}
	for _, exp := range expected {
		if !slices.Contains(names, exp) {
			t.Errorf("missing tenant subcommand: %s", exp)
		}
	}
}

func TestLockCommand_HasSubcommands(t *testing.T) {
	expected := []string{"set", "remove", "list"}
	names := make([]string, 0, len(expected))
	for _, cmd := range lockCmd.Commands() {
		names = append(names, cmd.Name())
	}
	for _, exp := range expected {
		if !slices.Contains(names, exp) {
			t.Errorf("missing lock subcommand: %s", exp)
		}
	}
}

func TestAuditCommand_HasSubcommands(t *testing.T) {
	expected := []string{"query", "usage", "unused"}
	names := make([]string, 0, len(expected))
	for _, cmd := range auditCmd.Commands() {
		names = append(names, cmd.Name())
	}
	for _, exp := range expected {
		if !slices.Contains(names, exp) {
			t.Errorf("missing audit subcommand: %s", exp)
		}
	}
}

// --- Argument validation ---

func TestSchemaGet_RequiresSchemaID(t *testing.T) {
	rootCmd.SetArgs([]string{"schema", "get"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "accepts 1 arg") {
		t.Errorf("expected error to contain %q, got %q", "accepts 1 arg", err.Error())
	}
}

func TestConfigGet_RequiresTenantAndField(t *testing.T) {
	rootCmd.SetArgs([]string{"config", "get", "only-one-arg"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "accepts 2 arg") {
		t.Errorf("expected error to contain %q, got %q", "accepts 2 arg", err.Error())
	}
}

func TestConfigSet_RequiresThreeArgs(t *testing.T) {
	rootCmd.SetArgs([]string{"config", "set", "tenant", "field"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "accepts 3 arg") {
		t.Errorf("expected error to contain %q, got %q", "accepts 3 arg", err.Error())
	}
}

func TestWatch_RequiresTenantID(t *testing.T) {
	rootCmd.SetArgs([]string{"watch"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "requires at least 1 arg") {
		t.Errorf("expected error to contain %q, got %q", "requires at least 1 arg", err.Error())
	}
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "NAME") {
		t.Errorf("expected output to contain %q", "NAME")
	}
	if !strings.Contains(output, "payments") {
		t.Errorf("expected output to contain %q", "payments")
	}
	if !strings.Contains(output, "settlement") {
		t.Errorf("expected output to contain %q", "settlement")
	}
	// Should have separator line.
	lines := strings.Split(output, "\n")
	if len(lines) < 4 {
		t.Errorf("got len %d, want >= %v", len(lines), 4)
	}
}

func TestPrintJSON(t *testing.T) {
	var buf bytes.Buffer
	data := map[string]string{"key": "value"}
	err := printJSON(&buf, data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), `"key": "value"`) {
		t.Errorf("expected %q to contain %q", buf.String(), `"key": "value"`)
	}
}

func TestPrintYAML(t *testing.T) {
	var buf bytes.Buffer
	data := map[string]string{"key": "value"}
	err := printYAML(&buf, data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "key: value") {
		t.Errorf("expected %q to contain %q", buf.String(), "key: value")
	}
}

func TestTableRows_Empty(t *testing.T) {
	rows := tableRows([]string{"A", "B"})
	if len(rows) != 1 {
		t.Errorf("got len %d, want %d", len(rows), 1)
	}
}

// --- Helpers ---

func TestParseDuration_Standard(t *testing.T) {
	d, err := parseDuration("24h")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := d; got != 24*time.Hour {
		t.Errorf("got %v, want %v", got, 24*time.Hour)
	}
}

func TestParseDuration_Days(t *testing.T) {
	d, err := parseDuration("7d")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := d; got != 7*24*time.Hour {
		t.Errorf("got %v, want %v", got, 7*24*time.Hour)
	}
}

func TestParseDuration_Invalid(t *testing.T) {
	_, err := parseDuration("abc")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
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
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(buf.String()) == 0 {
				t.Error("expected non-empty completion output")
			}
		})
	}
}
