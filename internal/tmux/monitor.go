package tmux

import (
	"strings"
	"time"
)

// MonitorLoop polls all agent panes at the given interval and updates window tab icons:
// needs approval → 🔴 wait; idle prompt → 🟢 done; processing → ⚡ busy.
// Runs until the process is killed.
func MonitorLoop(interval time.Duration) {
	if interval <= 0 {
		interval = 2 * time.Second
	}
	for {
		monitorTick()
		time.Sleep(interval)
	}
}

func monitorTick() {
	panes, err := ListPanes()
	if err != nil {
		return
	}

	// dedupe by window: one icon update per window (first agent pane wins)
	seen := map[string]bool{}
	for _, p := range panes {
		agentType := DetectAgent(p.Target)
		if agentType == "shell" {
			continue
		}
		winTarget := PaneToWindowTarget(p.Target)
		if seen[winTarget] {
			continue
		}
		seen[winTarget] = true
		setIconFromPaneState(p.Target, agentType)
	}
}

func setIconFromPaneState(paneTarget, agentType string) {
	output, err := ReadPane(paneTarget, 5)
	if err != nil {
		return
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) == 0 {
		return
	}

	// check last 3 lines to catch multi-line prompts
	checkLines := lines
	if len(lines) > 3 {
		checkLines = lines[len(lines)-3:]
	}

	// wait patterns have priority (agent needs user approval) → 🔴
	for _, line := range checkLines {
		norm := normalizeSpaces(line)
		for _, p := range waitPatterns[agentType] {
			if strings.Contains(norm, p) {
				_ = SetWindowStatus(paneTarget, "wait")
				return
			}
		}
	}

	// idle patterns: finished, ready for input → 🟢
	for _, line := range checkLines {
		norm := normalizeSpaces(line)
		for _, p := range idlePatterns[agentType] {
			if strings.Contains(norm, p) {
				_ = SetWindowStatus(paneTarget, "done")
				return
			}
		}
	}

	// still processing → ⚡
	_ = SetWindowStatus(paneTarget, "busy")
}
