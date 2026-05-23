package tmux

import (
	"fmt"
	"os/exec"
	"strings"
)

type Pane struct {
	Target  string
	Session string
	Window  string
	Index   string
	Command string
}

func ListPanes() ([]Pane, error) {
	out, err := exec.Command("tmux", "list-panes", "-a", "-F",
		"#{session_name}\t#{window_index}\t#{pane_index}\t#{pane_current_command}").Output()
	if err != nil {
		return nil, fmt.Errorf("tmux list-panes: %w", err)
	}

	var panes []Pane
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) < 4 {
			continue
		}
		p := Pane{
			Session: parts[0],
			Window:  parts[1],
			Index:   parts[2],
			Command: parts[3],
			Target:  fmt.Sprintf("%s:%s.%s", parts[0], parts[1], parts[2]),
		}
		panes = append(panes, p)
	}
	return panes, nil
}

func SendToPane(target, text string, enter bool) error {
	args := []string{"send-keys", "-t", target, text}
	if enter {
		args = append(args, "Enter")
	}
	if err := exec.Command("tmux", args...).Run(); err != nil {
		return fmt.Errorf("tmux send-keys to %s: %w", target, err)
	}
	return nil
}

func ReadPane(target string, lines int) (string, error) {
	out, err := exec.Command("tmux", "capture-pane", "-t", target, "-p",
		"-S", fmt.Sprintf("-%d", lines)).Output()
	if err != nil {
		return "", fmt.Errorf("tmux capture-pane %s: %w", target, err)
	}
	return string(out), nil
}

func Broadcast(text string, enter bool, filter string) (int, error) {
	panes, err := ListPanes()
	if err != nil {
		return 0, err
	}

	count := 0
	for _, p := range panes {
		if filter != "all" && !matchesFilter(p.Command, filter) {
			continue
		}
		if err := SendToPane(p.Target, text, enter); err != nil {
			continue
		}
		count++
	}
	return count, nil
}

// CreatePane creates a new tmux pane and returns its target (session:window.pane).
// If split is "h" or "v", splits the given target pane. Otherwise creates a new window.
func CreatePane(session, name, dir, command, split string) (string, error) {
	format := "#{session_name}:#{window_index}.#{pane_index}"

	var args []string
	if split == "h" || split == "v" {
		direction := "-h"
		if split == "v" {
			direction = "-v"
		}
		args = []string{"split-window", direction, "-t", session, "-P", "-F", format}
	} else {
		args = []string{"new-window", "-t", session, "-P", "-F", format}
		if name != "" {
			args = append(args, "-n", name)
		}
	}

	if dir != "" {
		args = append(args, "-c", dir)
	}

	if command != "" {
		args = append(args, command)
	}

	out, err := exec.Command("tmux", args...).Output()
	if err != nil {
		return "", fmt.Errorf("tmux create pane: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// RenameWindow renames a tmux window (tab), optionally prefixing with a status icon.
// status: "busy" → ⚡, "done" → ✓, "wait" → ⏳, "" → no prefix
func RenameWindow(target, name, status string) error {
	prefix := map[string]string{
		"busy": "⚡ ",
		"done": "🟢 ",
		"wait": "🔴 ",
	}[status]
	fullName := prefix + name
	// Disable auto-rename for this window so the icon persists
	_ = exec.Command("tmux", "set-window-option", "-t", target, "automatic-rename", "off").Run()
	if err := exec.Command("tmux", "rename-window", "-t", target, fullName).Run(); err != nil {
		return fmt.Errorf("tmux rename-window %s: %w", target, err)
	}
	return nil
}

// RenamePane sets a visible title on a pane (shown in the pane border).
func RenamePane(target, title string) error {
	if err := exec.Command("tmux", "select-pane", "-t", target, "-T", title).Run(); err != nil {
		return fmt.Errorf("tmux rename pane %s: %w", target, err)
	}
	return nil
}

func matchesFilter(cmd, filter string) bool {
	cmd = strings.ToLower(cmd)
	switch filter {
	case "claude":
		return strings.Contains(cmd, "claude")
	case "codex":
		return strings.Contains(cmd, "codex")
	case "cursor":
		return strings.Contains(cmd, "cursor")
	}
	return true
}
