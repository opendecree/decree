package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
)

// --- mustGetString / mustGetBool / mustGetInt32 ---

func TestMustGetString(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("name", "default", "")

	if got := mustGetString(cmd, "name"); got != "default" {
		t.Errorf("got %q, want %q", got, "default")
	}
	_ = cmd.Flags().Set("name", "hello")
	if got := mustGetString(cmd, "name"); got != "hello" {
		t.Errorf("got %q, want %q", got, "hello")
	}
}

func TestMustGetBool(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().Bool("flag", false, "")

	if got := mustGetBool(cmd, "flag"); got != false {
		t.Errorf("got %v, want false", got)
	}
	_ = cmd.Flags().Set("flag", "true")
	if got := mustGetBool(cmd, "flag"); got != true {
		t.Errorf("got %v, want true", got)
	}
}

func TestMustGetInt32(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().Int32("num", 0, "")

	if got := mustGetInt32(cmd, "num"); got != 0 {
		t.Errorf("got %v, want 0", got)
	}
	_ = cmd.Flags().Set("num", "42")
	if got := mustGetInt32(cmd, "num"); got != 42 {
		t.Errorf("got %v, want 42", got)
	}
}

// --- writeFileExclusive ---

func TestWriteFileExclusive_Creates(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.txt")

	if err := writeFileExclusive(path, []byte("hello"), false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read error: %v", err)
	}
	if string(data) != "hello" {
		t.Errorf("got %q, want %q", string(data), "hello")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat error: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("got perm %o, want 0600", perm)
	}
}

func TestWriteFileExclusive_NoForce_ExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.txt")
	_ = os.WriteFile(path, []byte("original"), 0o600)

	if err := writeFileExclusive(path, []byte("new"), false); err == nil {
		t.Error("expected error when file exists and force=false, got nil")
	}
}

func TestWriteFileExclusive_Force_Overwrites(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.txt")
	_ = os.WriteFile(path, []byte("original"), 0o600)

	if err := writeFileExclusive(path, []byte("new"), true); err != nil {
		t.Fatalf("unexpected error with force=true: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read error: %v", err)
	}
	if string(data) != "new" {
		t.Errorf("got %q, want %q", string(data), "new")
	}
}
