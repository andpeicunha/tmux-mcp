package tmux

import (
	"os/exec"
	"testing"
)

func TestPaneToWindowTarget(t *testing.T) {
	tests := map[string]string{
		"main:0.1":  "main:0",
		"0:0.0":     "0:0",
		"main:0":    "main:0",
	}
	for input, want := range tests {
		if got := PaneToWindowTarget(input); got != want {
			t.Errorf("PaneToWindowTarget(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestStripStatusIcon(t *testing.T) {
	tests := map[string]string{
		"⚡ executor":  "executor",
		"🟢 executor":  "executor",
		"🔴 executor":  "executor",
		"executor":     "executor",
	}
	for input, want := range tests {
		if got := StripStatusIcon(input); got != want {
			t.Errorf("StripStatusIcon(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestSetWindowStatusIntegration(t *testing.T) {
	if err := exec.Command("tmux", "list-sessions").Run(); err != nil {
		t.Skip("tmux not available")
	}

	target := "0:0.0"
	baseName := "icon-test"
	windowTarget := PaneToWindowTarget(target)

	if err := RenameWindow(windowTarget, baseName, ""); err != nil {
		t.Fatalf("setup rename: %v", err)
	}
	t.Cleanup(func() {
		_ = RenameWindow(windowTarget, baseName, "")
	})

	if err := SetWindowStatus(target, "busy"); err != nil {
		t.Fatalf("SetWindowStatus busy: %v", err)
	}
	name, err := GetWindowName(windowTarget)
	if err != nil {
		t.Fatalf("GetWindowName: %v", err)
	}
	if name != "⚡ "+baseName {
		t.Fatalf("busy icon = %q", name)
	}

	if err := SetWindowStatus(target, "done"); err != nil {
		t.Fatalf("SetWindowStatus done: %v", err)
	}
	name, err = GetWindowName(windowTarget)
	if err != nil {
		t.Fatalf("GetWindowName: %v", err)
	}
	if name != "🟢 "+baseName {
		t.Fatalf("done icon = %q", name)
	}
}
