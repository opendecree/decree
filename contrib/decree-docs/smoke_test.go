package main

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestSmoke_BinaryBuildsAndRuns is the CLI smoke test: the binary builds, and
// both `decree-docs --help` and `decree-docs version` exit 0.
func TestSmoke_BinaryBuildsAndRuns(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skipf("go binary not in PATH: %v", err)
	}

	bin := filepath.Join(t.TempDir(), "decree-docs")
	build := exec.Command("go", "build", "-o", bin, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build failed: %v\n%s", err, out)
	}

	for _, args := range [][]string{{"--help"}, {"version"}} {
		name := strings.Join(args, " ")
		t.Run(name, func(t *testing.T) {
			out, err := exec.Command(bin, args...).CombinedOutput()
			if err != nil {
				t.Fatalf("decree-docs %s failed: %v\n%s", name, err, out)
			}
			if !strings.Contains(string(out), "decree-docs") {
				t.Errorf("expected output of %q to mention decree-docs, got %q", name, out)
			}
		})
	}
}
