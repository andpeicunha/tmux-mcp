package tmux

import (
	"fmt"
	"os/exec"
	"strings"
	"time"
	"unicode"
)

// idlePatterns: agent finished and is ready for new input → 🟢
var idlePatterns = map[string][]string{
	"claude": {"❯", "? for shortcuts", "✓"},
	"codex":  {"❯", ">"},
	"cursor": {"Add a follow-up", "ctrl+c to stop"},
	"shell":  {"$", "%", "#"},
}

// waitPatterns: agent needs user approval before continuing → 🔴
var waitPatterns = map[string][]string{
	"cursor": {"Run (once)", "(y)", "allowlist?", "ctrl+r to review"},
	"claude": {"Do you want to proceed"},
}

// normalizeSpaces replaces all Unicode whitespace (including NBSP) with a regular space.
func normalizeSpaces(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return ' '
		}
		return r
	}, s)
}

func DetectAgent(target string) string {
	out, err := exec.Command("tmux", "display-message", "-t", target, "-p",
		"#{pane_current_command}").Output()
	if err != nil {
		return "shell"
	}
	cmd := strings.ToLower(strings.TrimSpace(string(out)))
	switch {
	case strings.Contains(cmd, "claude"):
		return "claude"
	case strings.Contains(cmd, "codex"):
		return "codex"
	case strings.Contains(cmd, "cursor"), cmd == "agent":
		return "cursor"
	default:
		return "shell"
	}
}

func WaitForIdle(target, agentType string, timeoutSecs int, updateIcon bool) (string, error) {
	if agentType == "auto" {
		agentType = DetectAgent(target)
	}

	patterns, ok := idlePatterns[agentType]
	if !ok {
		patterns = idlePatterns["shell"]
	}

	if updateIcon {
		_ = SetWindowStatus(target, "busy")
	}

	start := time.Now()
	deadline := time.Duration(timeoutSecs) * time.Second

	for time.Since(start) < deadline {
		output, err := ReadPane(target, 5)
		if err != nil {
			if updateIcon {
				_ = SetWindowStatus(target, "wait")
			}
			return "", err
		}

		lines := strings.Split(strings.TrimSpace(output), "\n")
		if len(lines) > 0 {
			lastLine := normalizeSpaces(lines[len(lines)-1])
			for _, pattern := range patterns {
				if strings.Contains(lastLine, pattern) {
					if updateIcon {
						_ = SetWindowStatus(target, "done")
					}
					return time.Since(start).Round(time.Millisecond).String(), nil
				}
			}
		}

		time.Sleep(500 * time.Millisecond)
	}

	if updateIcon {
		_ = SetWindowStatus(target, "wait")
	}
	return "", fmt.Errorf("timeout after %ds waiting for %s to become idle in %s", timeoutSecs, agentType, target)
}
