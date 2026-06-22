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
		_ = generateCmd.Flags().Set("flavor", "plain")
		_ = generateCmd.Flags().Set("pages", "single")
		_ = generateCmd.Flags().Set("out-dir", "")
		_ = generateCmd.Flags().Set("theme", "light")
		_ = generateCmd.Flags().Set("css", "")
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

func TestGenerate_FlavorFlagCompletion(t *testing.T) {
	complete, ok := generateCmd.GetFlagCompletionFunc("flavor")
	if !ok {
		t.Fatal("expected a completion function for --flavor")
	}
	values, _ := complete(generateCmd, nil, "")
	if !slices.Equal(values, mdFlavors) {
		t.Errorf("got completions %v, want %v", values, mdFlavors)
	}
}

func TestGenerate_PagesFlagCompletion(t *testing.T) {
	complete, ok := generateCmd.GetFlagCompletionFunc("pages")
	if !ok {
		t.Fatal("expected a completion function for --pages")
	}
	values, _ := complete(generateCmd, nil, "")
	if !slices.Equal(values, mdPageModes) {
		t.Errorf("got completions %v, want %v", values, mdPageModes)
	}
}

func TestGenerate_MD_SinglePageGoesToStdout(t *testing.T) {
	code, stdout, stderr := runGenerateCLI(t,
		"generate", "--file", "testdata/minimal.schema.yaml", "--format", "md")
	if code != 0 {
		t.Fatalf("got exit code %d, want 0 (stderr: %s)", code, stderr)
	}
	if !strings.HasPrefix(stdout, "# minimal\n\n") {
		t.Errorf("expected stdout to start with the schema heading, got %q", stdout)
	}
}

func TestGenerate_MD_UnknownFlavor(t *testing.T) {
	code, _, stderr := runGenerateCLI(t,
		"generate", "--file", "testdata/minimal.schema.yaml", "--format", "md", "--flavor", "fancy")
	if code != 1 {
		t.Errorf("got exit code %d, want 1", code)
	}
	if !strings.Contains(stderr, `unknown flavor "fancy"`) || !strings.Contains(stderr, "valid flavors: plain, material") {
		t.Errorf("expected unknown-flavor error, got %q", stderr)
	}
}

func TestGenerate_MD_UnknownPageMode(t *testing.T) {
	code, _, stderr := runGenerateCLI(t,
		"generate", "--file", "testdata/minimal.schema.yaml", "--format", "md", "--pages", "weekly")
	if code != 1 {
		t.Errorf("got exit code %d, want 1", code)
	}
	if !strings.Contains(stderr, `unknown pages mode "weekly"`) || !strings.Contains(stderr, "valid modes: single, multi") {
		t.Errorf("expected unknown-pages error, got %q", stderr)
	}
}

func TestGenerate_MD_MultiPageRequiresOutDir(t *testing.T) {
	code, stdout, stderr := runGenerateCLI(t,
		"generate", "--file", "testdata/full.schema.yaml", "--format", "md", "--pages", "multi")
	if code != 1 {
		t.Errorf("got exit code %d, want 1", code)
	}
	if !strings.Contains(stderr, "--out-dir is required") {
		t.Errorf("expected out-dir-required error, got %q", stderr)
	}
	if stdout != "" {
		t.Errorf("expected empty stdout, got %q", stdout)
	}
}

func TestGenerate_MD_MultiPageWritesFiles(t *testing.T) {
	outDir := t.TempDir()
	code, _, stderr := runGenerateCLI(t,
		"generate", "--file", "testdata/full.schema.yaml", "--format", "md", "--pages", "multi", "--out-dir", outDir)
	if code != 0 {
		t.Fatalf("got exit code %d, want 0 (stderr: %s)", code, stderr)
	}
	for _, name := range []string{"index.md", "payments.md"} {
		path := filepath.Join(outDir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("expected %s to be written: %v", path, err)
			continue
		}
		if len(data) == 0 {
			t.Errorf("expected %s to be non-empty", path)
		}
	}
}

func TestGenerate_MD_OutDirNotADirectory(t *testing.T) {
	outDir := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(outDir, []byte("x"), 0o644); err != nil {
		t.Fatalf("write blocker file: %v", err)
	}
	code, _, stderr := runGenerateCLI(t,
		"generate", "--file", "testdata/full.schema.yaml", "--format", "md", "--pages", "multi", "--out-dir", outDir)
	if code != 1 {
		t.Errorf("got exit code %d, want 1", code)
	}
	if !strings.Contains(stderr, "create out-dir") {
		t.Errorf("expected create-out-dir error, got %q", stderr)
	}
}

func TestGenerate_MD_WriteFileError(t *testing.T) {
	outDir := t.TempDir()
	// index.md as a directory makes the write to that path fail.
	if err := os.Mkdir(filepath.Join(outDir, "index.md"), 0o755); err != nil {
		t.Fatalf("mkdir blocker: %v", err)
	}
	code, _, stderr := runGenerateCLI(t,
		"generate", "--file", "testdata/full.schema.yaml", "--format", "md", "--pages", "multi", "--out-dir", outDir)
	if code != 1 {
		t.Errorf("got exit code %d, want 1", code)
	}
	if !strings.Contains(stderr, "write") {
		t.Errorf("expected write error, got %q", stderr)
	}
}

func TestGenerate_MD_SinglePageWithOutDirWritesIndexFile(t *testing.T) {
	outDir := t.TempDir()
	code, stdout, stderr := runGenerateCLI(t,
		"generate", "--file", "testdata/minimal.schema.yaml", "--format", "md", "--out-dir", outDir)
	if code != 0 {
		t.Fatalf("got exit code %d, want 0 (stderr: %s)", code, stderr)
	}
	if stdout != "" {
		t.Errorf("expected empty stdout when writing to --out-dir, got %q", stdout)
	}
	if _, err := os.Stat(filepath.Join(outDir, "index.md")); err != nil {
		t.Errorf("expected index.md to be written: %v", err)
	}
}

func TestGenerate_ThemeFlagCompletion(t *testing.T) {
	complete, ok := generateCmd.GetFlagCompletionFunc("theme")
	if !ok {
		t.Fatal("expected a completion function for --theme")
	}
	values, _ := complete(generateCmd, nil, "")
	if !slices.Equal(values, htmlThemes) {
		t.Errorf("got completions %v, want %v", values, htmlThemes)
	}
}

func TestGenerate_HTML_SinglePageGoesToStdout(t *testing.T) {
	code, stdout, stderr := runGenerateCLI(t,
		"generate", "--file", "testdata/minimal.schema.yaml", "--format", "html")
	if code != 0 {
		t.Fatalf("got exit code %d, want 0 (stderr: %s)", code, stderr)
	}
	if !strings.HasPrefix(stdout, "<!DOCTYPE html>") {
		t.Errorf("expected stdout to start with the HTML doctype, got %q", stdout)
	}
}

func TestGenerate_HTML_UnknownTheme(t *testing.T) {
	code, _, stderr := runGenerateCLI(t,
		"generate", "--file", "testdata/minimal.schema.yaml", "--format", "html", "--theme", "neon")
	if code != 1 {
		t.Errorf("got exit code %d, want 1", code)
	}
	if !strings.Contains(stderr, `unknown theme "neon"`) || !strings.Contains(stderr, "valid themes: light, dark, auto") {
		t.Errorf("expected unknown-theme error, got %q", stderr)
	}
}

func TestGenerate_HTML_CSSFileNotFound(t *testing.T) {
	code, _, stderr := runGenerateCLI(t,
		"generate", "--file", "testdata/minimal.schema.yaml", "--format", "html", "--css", "testdata/does-not-exist.css")
	if code != 1 {
		t.Errorf("got exit code %d, want 1", code)
	}
	if !strings.Contains(stderr, "read --css file") {
		t.Errorf("expected read error, got %q", stderr)
	}
}

func TestGenerate_HTML_CSSOverrideApplied(t *testing.T) {
	cssPath := filepath.Join(t.TempDir(), "brand.css")
	if err := os.WriteFile(cssPath, []byte(":root { --decree-accent: #7c3aed; }"), 0o644); err != nil {
		t.Fatalf("write css fixture: %v", err)
	}
	code, stdout, stderr := runGenerateCLI(t,
		"generate", "--file", "testdata/minimal.schema.yaml", "--format", "html", "--css", cssPath)
	if code != 0 {
		t.Fatalf("got exit code %d, want 0 (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stdout, "@layer decree.user {\n:root { --decree-accent: #7c3aed; }\n}") {
		t.Errorf("expected user CSS to be appended in the decree.user layer, got %q", stdout)
	}
}

func TestGenerate_HTML_OutDirWritesIndexFile(t *testing.T) {
	outDir := t.TempDir()
	code, stdout, stderr := runGenerateCLI(t,
		"generate", "--file", "testdata/minimal.schema.yaml", "--format", "html", "--out-dir", outDir)
	if code != 0 {
		t.Fatalf("got exit code %d, want 0 (stderr: %s)", code, stderr)
	}
	if stdout != "" {
		t.Errorf("expected empty stdout when writing to --out-dir, got %q", stdout)
	}
	if _, err := os.Stat(filepath.Join(outDir, "index.html")); err != nil {
		t.Errorf("expected index.html to be written: %v", err)
	}
}

func TestGenerate_HTML_OutDirNotADirectory(t *testing.T) {
	outDir := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(outDir, []byte("x"), 0o644); err != nil {
		t.Fatalf("write blocker file: %v", err)
	}
	code, _, stderr := runGenerateCLI(t,
		"generate", "--file", "testdata/minimal.schema.yaml", "--format", "html", "--out-dir", outDir)
	if code != 1 {
		t.Errorf("got exit code %d, want 1", code)
	}
	if !strings.Contains(stderr, "create out-dir") {
		t.Errorf("expected create-out-dir error, got %q", stderr)
	}
}

func TestGenerate_HTML_WriteFileError(t *testing.T) {
	outDir := t.TempDir()
	// index.html as a directory makes the write to that path fail.
	if err := os.Mkdir(filepath.Join(outDir, "index.html"), 0o755); err != nil {
		t.Fatalf("mkdir blocker: %v", err)
	}
	code, _, stderr := runGenerateCLI(t,
		"generate", "--file", "testdata/minimal.schema.yaml", "--format", "html", "--out-dir", outDir)
	if code != 1 {
		t.Errorf("got exit code %d, want 1", code)
	}
	if !strings.Contains(stderr, "write") {
		t.Errorf("expected write error, got %q", stderr)
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
