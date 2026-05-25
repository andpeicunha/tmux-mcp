package tmux

import (
	"strings"
	"time"
)

// MonitorLoop polls all agent panes at the given interval and updates window tab icons:
// idle prompt → 🟢 done; still processing → ⚡ busy.
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
	patterns := idlePatterns[agentType]
	if len(patterns) == 0 {
		return
	}
	output, err := ReadPane(paneTarget, 5)
	if err != nil {
		return
	}
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) == 0 {
		return
	}
	lastLine := lines[len(lines)-1]
	for _, p := range patterns {
		if strings.Contains(lastLine, p) {
			_ = SetWindowStatus(paneTarget, "done") // 🟢 idle, waiting for input
			return
		}
	}
	_ = SetWindowStatus(paneTarget, "busy") // ⚡ processing
}
