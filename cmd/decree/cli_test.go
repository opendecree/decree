package main

import (
	"bytes"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/opendecree/decree/sdk/adminclient"
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
	expected := []string{"query", "usage", "unused", "verify"}
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

// --- audit verify exit code ---

func TestPrintVerifyResult_OK(t *testing.T) {
	var buf bytes.Buffer
	result := adminclient.VerifyChainResult{OK: true, Total: 5}
	err := printVerifyResult(&buf, result)
	if err != nil {
		t.Errorf("expected nil error for intact chain, got %v", err)
	}
	if !strings.Contains(buf.String(), "OK") {
		t.Errorf("expected output to contain %q, got %q", "OK", buf.String())
	}
}

func TestPrintVerifyResult_BrokenChain(t *testing.T) {
	var buf bytes.Buffer
	result := adminclient.VerifyChainResult{
		OK:    false,
		Total: 3,
		Breaks: []adminclient.VerifyChainBreak{
			{Position: 2, EntryID: "entry-abc", Got: "badhash", Want: "goodhash"},
		},
	}
	err := printVerifyResult(&buf, result)
	if err == nil {
		t.Fatal("expected non-nil error for broken chain, got nil")
	}
	if !strings.Contains(err.Error(), "break") {
		t.Errorf("expected error to mention breaks, got %q", err.Error())
	}
	out := buf.String()
	if !strings.Contains(out, "FAIL") {
		t.Errorf("expected output to contain %q, got %q", "FAIL", out)
	}
	if !strings.Contains(out, "entry-abc") {
		t.Errorf("expected output to contain entry ID %q, got %q", "entry-abc", out)
	}
}

func TestPrintVerifyResult_TablePrintedBeforeError(t *testing.T) {
	// Ensure the table is printed even when breaks are found (non-zero exit must
	// not suppress output).
	var buf bytes.Buffer
	result := adminclient.VerifyChainResult{
		OK:    false,
		Total: 1,
		Breaks: []adminclient.VerifyChainBreak{
			{Position: 1, EntryID: "id-1", Got: "aaa", Want: "bbb"},
		},
	}
	err := printVerifyResult(&buf, result)
	if err == nil {
		t.Fatal("expected error for broken chain")
	}
	if buf.Len() == 0 {
		t.Error("expected table output before error, got empty buffer")
	}
}

// --- Argument validation ---

// resetRootCmd restores rootCmd I/O and args after a test so that global state
// mutations do not leak between tests.
func resetRootCmd(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		rootCmd.SetArgs(nil)
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
	})
}

func TestSchemaGet_RequiresSchemaID(t *testing.T) {
	resetRootCmd(t)
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
	resetRootCmd(t)
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
	resetRootCmd(t)
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
	resetRootCmd(t)
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

// --- SilenceUsage / SilenceErrors (#707) ---

func TestRootCmd_SilenceUsage(t *testing.T) {
	if !rootCmd.SilenceUsage {
		t.Error("expected rootCmd.SilenceUsage = true, got false")
	}
}

func TestRootCmd_SilenceErrors(t *testing.T) {
	if !rootCmd.SilenceErrors {
		t.Error("expected rootCmd.SilenceErrors = true, got false")
	}
}

// --- validate RunE returns error instead of os.Exit (#708) ---

// TestValidateCmd_MissingFlagsReturnsError verifies that validateCmd.RunE
// returns an error (rather than calling os.Exit) when required flags are absent.
func TestValidateCmd_MissingFlagsReturnsError(t *testing.T) {
	resetRootCmd(t)
	rootCmd.SetArgs([]string{"validate"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing --schema / --config, got nil")
	}
	if !strings.Contains(err.Error(), "required") {
		t.Errorf("expected error to mention %q, got %q", "required", err.Error())
	}
}

// TestValidateCmd_InvalidSchemaFileReturnsError verifies that a missing schema
// file returns an error from RunE instead of calling os.Exit.
func TestValidateCmd_InvalidSchemaFileReturnsError(t *testing.T) {
	resetRootCmd(t)
	rootCmd.SetArgs([]string{"validate", "--schema", "/nonexistent/schema.yaml", "--config", "/nonexistent/config.yaml"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing schema file, got nil")
	}
}

// --- Exit codes and stderr/stdout routing ---

// TestArgError_GoesToStderr verifies that a cobra argument-validation error
// (wrong number of positional args) is returned by Execute and that nothing
// is written to stdout.
func TestArgError_GoesToStderr(t *testing.T) {
	resetRootCmd(t)
	var stdout, stderr bytes.Buffer
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stderr)
	rootCmd.SetArgs([]string{"schema", "get"}) // requires exactly 1 arg

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if stdout.Len() != 0 {
		t.Errorf("expected empty stdout on argument error, got: %q", stdout.String())
	}
}

// TestArgError_ReturnsNonNilError verifies that Execute returns a non-nil
// error (i.e. the exit path is via returned error, not os.Exit).
func TestArgError_ReturnsNonNilError(t *testing.T) {
	resetRootCmd(t)
	var stdout, stderr bytes.Buffer
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stderr)
	rootCmd.SetArgs([]string{"config", "get", "only-one-arg"}) // requires 2 args

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected non-nil error for wrong arg count, got nil")
	}
}

// TestSchemaCreate_MissingFile_ErrorNotOnStdout verifies that a RunE error
// (missing required flag) is returned and stdout stays empty.
func TestSchemaCreate_MissingFile_ErrorNotOnStdout(t *testing.T) {
	resetRootCmd(t)
	var stdout, stderr bytes.Buffer
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stderr)
	rootCmd.SetArgs([]string{"schema", "create"}) // --file is required

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing --file, got nil")
	}
	if !strings.Contains(err.Error(), "--file") {
		t.Errorf("expected error to mention --file, got %q", err.Error())
	}
	if stdout.Len() != 0 {
		t.Errorf("expected empty stdout on flag error, got: %q", stdout.String())
	}
}

// TestTenantCreate_MissingFlags_ReturnsError verifies that tenant create
// without required flags returns an error rather than calling os.Exit.
func TestTenantCreate_MissingFlags_ReturnsError(t *testing.T) {
	resetRootCmd(t)
	var stdout, stderr bytes.Buffer
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stderr)
	rootCmd.SetArgs([]string{"tenant", "create"}) // --name, --schema, --schema-version required

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing tenant create flags, got nil")
	}
	if stdout.Len() != 0 {
		t.Errorf("expected empty stdout on flag error, got: %q", stdout.String())
	}
}

// TestValidateCmd_MissingFlags_StdoutEmpty verifies that a validate error
// (missing flags) does not pollute stdout.
func TestValidateCmd_MissingFlags_StdoutEmpty(t *testing.T) {
	resetRootCmd(t)
	var stdout, stderr bytes.Buffer
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stderr)
	rootCmd.SetArgs([]string{"validate"})

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing --schema / --config, got nil")
	}
	if stdout.Len() != 0 {
		t.Errorf("expected empty stdout on validation flag error, got: %q", stdout.String())
	}
}

// TestIsolatedCommand_FreshState verifies that creating a fresh cobra.Command
// (not the shared rootCmd) keeps tests fully isolated from global flag state.
func TestIsolatedCommand_FreshState(t *testing.T) {
	// Build a minimal isolated command tree to confirm the isolation pattern works.
	parent := &cobra.Command{Use: "root", SilenceUsage: true, SilenceErrors: true}
	child := &cobra.Command{
		Use:  "sub",
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, _ []string) error { return nil },
	}
	parent.AddCommand(child)

	var stdout, stderr bytes.Buffer
	parent.SetOut(&stdout)
	parent.SetErr(&stderr)
	parent.SetArgs([]string{"sub"}) // missing required arg

	err := parent.Execute()
	if err == nil {
		t.Fatal("expected error for missing positional arg, got nil")
	}
	if stdout.Len() != 0 {
		t.Errorf("expected empty stdout, got: %q", stdout.String())
	}
}

// --- Status message routing ---

func TestPrintStatus_WritesToStderrNotStdout(t *testing.T) {
	var stdout, stderr bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	printStatus(cmd, "Deleted.\n")

	if stdout.Len() != 0 {
		t.Errorf("expected empty stdout, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "Deleted.") {
		t.Errorf("expected stderr to contain %q, got %q", "Deleted.", stderr.String())
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
