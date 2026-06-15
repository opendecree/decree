package main

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// -update rewrites the golden files from current output:
//
//	go test . -run TestGenerate_JSONGolden -update
var update = flag.Bool("update", false, "rewrite golden files")

// resetGenerateCmd restores rootCmd I/O and generate flag values after a
// test; cobra flag values persist across Execute calls.
func resetGenerateCmd(t *testing.T) {
	t.Helper()
	resetRootCmd(t)
	t.Cleanup(func() {
		_ = generateCmd.Flags().Set("file", "")
		_ = generateCmd.Flags().Set("format", "json")
	})
}

// runGenerateCLI drives `decree-docs <args...>` in-process and returns the
// exit code with captured stdout and stderr.
func runGenerateCLI(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	resetGenerateCmd(t)
	var out, errOut bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&errOut)
	code = run(args)
	return code, out.String(), errOut.String()
}

func TestRootCommand_HasGenerateSubcommand(t *testing.T) {
	names := make([]string, 0, len(rootCmd.Commands()))
	for _, cmd := range rootCmd.Commands() {
		names = append(names, cmd.Name())
	}
	if !slices.Contains(names, "generate") {
		t.Errorf("missing subcommand: generate (got %v)", names)
	}
}

func TestGenerate_FormatFlagCompletion(t *testing.T) {
	complete, ok := generateCmd.GetFlagCompletionFunc("format")
	if !ok {
		t.Fatal("expected a completion function for --format")
	}
	values, _ := complete(generateCmd, nil, "")
	if !slices.Equal(values, docFormats) {
		t.Errorf("got completions %v, want %v", values, docFormats)
	}
}

// TestGenerate_JSONGolden is the end-to-end check for the stable JSON shape:
// generating from the fixture schemas must reproduce the golden files byte
// for byte.
func TestGenerate_JSONGolden(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		golden string
	}{
		{
			name:   "full schema",
			args:   []string{"generate", "--file", "testdata/full.schema.yaml", "--format", "json"},
			golden: "testdata/full.golden.json",
		},
		{
			name: "minimal schema with default format",
			// No --format: json is the default.
			args:   []string{"generate", "--file", "testdata/minimal.schema.yaml"},
			golden: "testdata/minimal.golden.json",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, stdout, stderr := runGenerateCLI(t, tt.args...)
			if code != 0 {
				t.Fatalf("got exit code %d, want 0 (stderr: %s)", code, stderr)
			}
			if *update {
				if err := os.WriteFile(tt.golden, []byte(stdout), 0o644); err != nil {
					t.Fatalf("update golden: %v", err)
				}
			}
			want, err := os.ReadFile(tt.golden)
			if err != nil {
				t.Fatalf("read golden: %v", err)
			}
			if stdout != string(want) {
				t.Errorf("output does not match %s:\ngot:\n%s\nwant:\n%s", tt.golden, stdout, want)
			}
		})
	}
}

func TestGenerate_UnknownFormat(t *testing.T) {
	code, stdout, stderr := runGenerateCLI(t,
		"generate", "--file", "testdata/minimal.schema.yaml", "--format", "yaml")
	if code != 1 {
		t.Errorf("got exit code %d, want 1", code)
	}
	if !strings.Contains(stderr, `unknown format "yaml"`) || !strings.Contains(stderr, "valid formats: json") {
		t.Errorf("expected unknown-format error listing valid formats, got %q", stderr)
	}
	if stdout != "" {
		t.Errorf("expected empty stdout, got %q", stdout)
	}
}

func TestGenerate_NoSource(t *testing.T) {
	code, _, stderr := runGenerateCLI(t, "generate")
	if code != 1 {
		t.Errorf("got exit code %d, want 1", code)
	}
	if !strings.Contains(stderr, "--file") {
		t.Errorf("expected error to point at --file, got %q", stderr)
	}
}

func TestGenerate_ServerModeUnavailable(t *testing.T) {
	code, _, stderr := runGenerateCLI(t, "generate", "some-schema-id")
	if code != 1 {
		t.Errorf("got exit code %d, want 1", code)
	}
	if !strings.Contains(stderr, "server mode") || !strings.Contains(stderr, "--file") {
		t.Errorf("expected server-mode-unavailable error, got %q", stderr)
	}
}

func TestGenerate_FileAndSchemaIDMutuallyExclusive(t *testing.T) {
	code, _, stderr := runGenerateCLI(t,
		"generate", "some-schema-id", "--file", "testdata/minimal.schema.yaml")
	if code != 1 {
		t.Errorf("got exit code %d, want 1", code)
	}
	if !strings.Contains(stderr, "mutually exclusive") {
		t.Errorf("expected mutual-exclusion error, got %q", stderr)
	}
}

func TestGenerate_FileNotFound(t *testing.T) {
	code, _, stderr := runGenerateCLI(t, "generate", "--file", "testdata/does-not-exist.yaml")
	if code != 1 {
		t.Errorf("got exit code %d, want 1", code)
	}
	if !strings.Contains(stderr, "read schema file") {
		t.Errorf("expected read error, got %q", stderr)
	}
}

func TestGenerate_InvalidSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.yaml")
	if err := os.WriteFile(path, []byte("name: no-spec-version\n"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	code, _, stderr := runGenerateCLI(t, "generate", "--file", path)
	if code != 1 {
		t.Errorf("got exit code %d, want 1", code)
	}
	if !strings.Contains(stderr, "invalid schema YAML") {
		t.Errorf("expected schema validation error, got %q", stderr)
	}
}
