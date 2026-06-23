package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestRootCommand_HasGenManSubcommand(t *testing.T) {
	names := make([]string, 0, len(rootCmd.Commands()))
	for _, cmd := range rootCmd.Commands() {
		names = append(names, cmd.Name())
	}
	if !slices.Contains(names, "gen-man") {
		t.Errorf("missing subcommand: gen-man (got %v)", names)
	}
}

func TestGenManCmd_Hidden(t *testing.T) {
	if !genManCmd.Hidden {
		t.Error("expected genManCmd.Hidden = true, got false")
	}
}

func TestRootCmd_DisableAutoGenTag(t *testing.T) {
	if !rootCmd.DisableAutoGenTag {
		t.Error("expected rootCmd.DisableAutoGenTag = true, got false")
	}
}

// TestGenMan_DefaultOutDir runs gen-man with no args and checks man pages for
// root, generate, and version land in the default docs/man directory,
// relative to a temp working directory.
func TestGenMan_DefaultOutDir(t *testing.T) {
	resetRootCmd(t)

	dir := t.TempDir()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })

	code, stderr := runGenManCLI(t, "gen-man")
	if code != 0 {
		t.Fatalf("got exit code %d, want 0 (stderr: %q)", code, stderr)
	}
	if !strings.Contains(stderr, "docs/man") {
		t.Errorf("expected stderr to mention docs/man, got %q", stderr)
	}

	for _, name := range []string{"decree-docs.1", "decree-docs-generate.1", "decree-docs-version.1"} {
		path := filepath.Join(dir, "docs", "man", name)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected man page %s to exist: %v", path, err)
		}
	}
}

// TestGenMan_CustomOutDir checks the optional positional output-dir arg is
// honored and man pages are written there instead of the default.
func TestGenMan_CustomOutDir(t *testing.T) {
	resetRootCmd(t)

	dir := t.TempDir()
	outDir := filepath.Join(dir, "custom-man")

	code, stderr := runGenManCLI(t, "gen-man", outDir)
	if code != 0 {
		t.Fatalf("got exit code %d, want 0 (stderr: %q)", code, stderr)
	}

	for _, name := range []string{"decree-docs.1", "decree-docs-generate.1", "decree-docs-version.1"} {
		path := filepath.Join(outDir, name)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected man page %s to exist: %v", path, err)
		}
	}
}

// TestGenMan_UnwritableOutDir exercises the MkdirAll error path by pointing
// the output dir at a path nested under a file (not a directory).
func TestGenMan_UnwritableOutDir(t *testing.T) {
	resetRootCmd(t)

	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	outDir := filepath.Join(blocker, "man")

	code, stderr := runGenManCLI(t, "gen-man", outDir)
	if code != 1 {
		t.Fatalf("got exit code %d, want 1", code)
	}
	if !strings.Contains(stderr, "create output dir") {
		t.Errorf("expected stderr to mention create output dir, got %q", stderr)
	}
}

// runGenManCLI drives `decree-docs <args...>` in-process and returns the
// exit code with captured stderr.
func runGenManCLI(t *testing.T, args ...string) (code int, stderr string) {
	t.Helper()
	var out, errOut strings.Builder
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&errOut)
	code = run(args)
	return code, errOut.String()
}
