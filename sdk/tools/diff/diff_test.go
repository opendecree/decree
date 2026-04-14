package diff

import (
	"testing"
)

func TestCompare_Identical(t *testing.T) {
	m := map[string]string{"a": "1", "b": "2"}
	r := Compare(m, m)
	if r.HasChanges() {
		t.Errorf("expected no changes, got %d", len(r.Changes))
	}
}

func TestCompare_Empty(t *testing.T) {
	r := Compare(nil, nil)
	if r.HasChanges() {
		t.Error("expected no changes for nil maps")
	}
}

func TestCompare_Added(t *testing.T) {
	old := map[string]string{"a": "1"}
	new := map[string]string{"a": "1", "b": "2"}
	r := Compare(old, new)

	if len(r.Changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(r.Changes))
	}
	c := r.Changes[0]
	if c.Type != Added || c.Path != "b" || c.NewValue != "2" {
		t.Errorf("unexpected change: %+v", c)
	}
}

func TestCompare_Removed(t *testing.T) {
	old := map[string]string{"a": "1", "b": "2"}
	new := map[string]string{"a": "1"}
	r := Compare(old, new)

	if len(r.Changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(r.Changes))
	}
	c := r.Changes[0]
	if c.Type != Removed || c.Path != "b" || c.OldValue != "2" {
		t.Errorf("unexpected change: %+v", c)
	}
}

func TestCompare_Modified(t *testing.T) {
	old := map[string]string{"a": "1"}
	new := map[string]string{"a": "2"}
	r := Compare(old, new)

	if len(r.Changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(r.Changes))
	}
	c := r.Changes[0]
	if c.Type != Modified || c.OldValue != "1" || c.NewValue != "2" {
		t.Errorf("unexpected change: %+v", c)
	}
}

func TestCompare_Mixed(t *testing.T) {
	old := map[string]string{"a": "1", "b": "2", "c": "3"}
	new := map[string]string{"a": "1", "b": "changed", "d": "4"}
	r := Compare(old, new)

	if len(r.Changes) != 3 {
		t.Fatalf("expected 3 changes, got %d", len(r.Changes))
	}

	// Results are sorted by path.
	assertChange(t, r.Changes[0], "b", Modified)
	assertChange(t, r.Changes[1], "c", Removed)
	assertChange(t, r.Changes[2], "d", Added)
}

func TestByType(t *testing.T) {
	old := map[string]string{"a": "1", "b": "2"}
	new := map[string]string{"b": "changed", "c": "3"}
	r := Compare(old, new)

	if len(r.ByType(Added)) != 1 {
		t.Errorf("expected 1 added, got %d", len(r.ByType(Added)))
	}
	if len(r.ByType(Removed)) != 1 {
		t.Errorf("expected 1 removed, got %d", len(r.ByType(Removed)))
	}
	if len(r.ByType(Modified)) != 1 {
		t.Errorf("expected 1 modified, got %d", len(r.ByType(Modified)))
	}
}

func TestFormat_NoChanges(t *testing.T) {
	r := Compare(nil, nil)
	if r.Format() != "no changes" {
		t.Errorf("unexpected format: %q", r.Format())
	}
}

func TestFormat_WithChanges(t *testing.T) {
	old := map[string]string{"a": "1"}
	new := map[string]string{"a": "2", "b": "3"}
	r := Compare(old, new)

	out := r.Format()
	if out == "" || out == "no changes" {
		t.Error("expected formatted output")
	}
}

func TestChangeType_String(t *testing.T) {
	tests := []struct {
		t    ChangeType
		want string
	}{
		{Added, "added"},
		{Removed, "removed"},
		{Modified, "modified"},
		{ChangeType(99), "unknown"},
	}
	for _, tt := range tests {
		if got := tt.t.String(); got != tt.want {
			t.Errorf("ChangeType(%d).String() = %q, want %q", tt.t, got, tt.want)
		}
	}
}

func assertChange(t *testing.T, c FieldChange, path string, typ ChangeType) {
	t.Helper()
	if c.Path != path || c.Type != typ {
		t.Errorf("expected %s %s, got %s %s", path, typ, c.Path, c.Type)
	}
}
