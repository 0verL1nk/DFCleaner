package platform

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetCommonTargets(t *testing.T) {
	home := "/tmp/testhome"
	targets := getCommonTargets(home)
	if len(targets) != 3 {
		t.Fatalf("expected 3 common targets, got %d", len(targets))
	}

	expected := map[string]string{
		filepath.Join(home, "Downloads"):   "Downloads",
		filepath.Join(home, ".cache"):      "Cache",
		filepath.Join(home, ".thumbnails"): "Thumbnails",
	}

	for _, t2 := range targets {
		if label, ok := expected[t2.Path]; !ok {
			t.Errorf("unexpected target path: %s", t2.Path)
		} else if t2.Label != label {
			t.Errorf("target %s: expected label %q, got %q", t2.Path, label, t2.Label)
		}
	}
}

func TestGetCommonTargetsEmptyHome(t *testing.T) {
	targets := getCommonTargets("")
	if targets != nil {
		t.Errorf("expected nil for empty home, got %v", targets)
	}
}

func TestGetQuickTargetsFiltersNonexistent(t *testing.T) {
	home := t.TempDir()
	os.MkdirAll(filepath.Join(home, "Downloads"), 0755)

	t.Setenv("HOME", home)
	targets := GetQuickTargets()

	for _, t2 := range targets {
		if _, err := os.Stat(t2.Path); err != nil {
			t.Errorf("GetQuickTargets returned nonexistent path: %s", t2.Path)
		}
	}
}

func TestGetDrivesReturnsRoot(t *testing.T) {
	drives := GetDrives()
	if len(drives) == 0 {
		t.Fatal("expected at least root drive")
	}

	found := false
	for _, d := range drives {
		if d.Path == "/" {
			found = true
			if !d.IsSystem {
				t.Error("root drive should be marked as system")
			}
			if d.Total == 0 {
				t.Error("root drive total should be > 0")
			}
		}
	}
	if !found {
		t.Error("root drive not found")
	}
}
