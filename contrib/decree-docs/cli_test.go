package main

import (
	"bytes"
	"slices"
	"strings"
	"testing"
)

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

// --- Command structure ---

func TestRootCommand_HasVersionSubcommand(t *testing.T) {
	names := make([]string, 0, len(rootCmd.Commands()))
	for _, cmd := range rootCmd.Commands() {
		names = append(names, cmd.Name())
	}
	if !slices.Contains(names, "version") {
		t.Errorf("missing subcommand: version (got %v)", names)
	}
}

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

// --- Exit codes ---

func TestRun_Help_ExitsZero(t *testing.T) {
	resetRootCmd(t)
	var stdout, stderr bytes.Buffer
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stderr)

	if code := run([]string{"--help"}); code != 0 {
		t.Errorf("got exit code %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "decree-docs") {
		t.Errorf("expected help output to mention decree-docs, got %q", stdout.String())
	}
}

func TestRun_NoArgs_ShowsHelp(t *testing.T) {
	resetRootCmd(t)
	var stdout, stderr bytes.Buffer
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stderr)

	if code := run([]string{}); code != 0 {
		t.Errorf("got exit code %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "Usage:") {
		t.Errorf("expected bare invocation to print help, got %q", stdout.String())
	}
}

func TestRun_VersionCommand_ExitsZero(t *testing.T) {
	resetRootCmd(t)
	var stdout, stderr bytes.Buffer
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stderr)

	if code := run([]string{"version"}); code != 0 {
		t.Errorf("got exit code %d, want 0", code)
	}
	want := "decree-docs " + version
	if !strings.Contains(stdout.String(), want) {
		t.Errorf("expected version output to contain %q, got %q", want, stdout.String())
	}
}

func TestRun_UnknownCommand_ExitsNonZeroWithError(t *testing.T) {
	resetRootCmd(t)
	var stdout, stderr bytes.Buffer
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stderr)

	if code := run([]string{"definitely-not-a-command"}); code != 1 {
		t.Errorf("got exit code %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "Error:") {
		t.Errorf("expected error message on stderr, got %q", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Errorf("expected empty stdout on unknown command, got %q", stdout.String())
	}
}
