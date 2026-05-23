package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/andrecunha/tmux-mcp/internal/tmux"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	s := server.NewMCPServer("tmux-mcp", "0.1.0",
		server.WithToolCapabilities(true),
	)

	s.AddTool(mcp.NewTool("list_panes",
		mcp.WithDescription("List all tmux panes across all sessions, showing target, session, window, pane index and running command"),
	), handleListPanes)

	s.AddTool(mcp.NewTool("send_to_pane",
		mcp.WithDescription("Send text or a command to a specific tmux pane. Use this to instruct an agent running in another pane."),
		mcp.WithString("target",
			mcp.Required(),
			mcp.Description("Pane target in session:window.pane format, e.g. main:0.1"),
		),
		mcp.WithString("text",
			mcp.Required(),
			mcp.Description("Text or command to send to the pane"),
		),
		mcp.WithBoolean("enter",
			mcp.Description("Press Enter after sending (default true)"),
		),
	), handleSendToPane)

	s.AddTool(mcp.NewTool("read_pane",
		mcp.WithDescription("Read the current output of a tmux pane. Use this to get the result from an agent."),
		mcp.WithString("target",
			mcp.Required(),
			mcp.Description("Pane target in session:window.pane format"),
		),
		mcp.WithNumber("lines",
			mcp.Description("Number of lines to read from the end (default 50)"),
		),
	), handleReadPane)

	s.AddTool(mcp.NewTool("broadcast",
		mcp.WithDescription("Send text to multiple panes at once, optionally filtered by agent type"),
		mcp.WithString("text",
			mcp.Required(),
			mcp.Description("Text to broadcast"),
		),
		mcp.WithBoolean("enter",
			mcp.Description("Press Enter after sending (default true)"),
		),
		mcp.WithString("filter",
			mcp.Description("Filter panes by agent type: claude, codex, cursor, or all (default all)"),
		),
	), handleBroadcast)

	s.AddTool(mcp.NewTool("rename_window",
		mcp.WithDescription("Rename a tmux window (tab) with an optional status icon visible in the status bar. Use this to signal agent state to the user."),
		mcp.WithString("target",
			mcp.Required(),
			mcp.Description("Window target in session:window format, e.g. main:1"),
		),
		mcp.WithString("name",
			mcp.Required(),
			mcp.Description("Window name, e.g. 'executor', 'reviewer'"),
		),
		mcp.WithString("status",
			mcp.Description("Status icon to prefix: 'busy' (⚡), 'done' (🟢), 'wait' (🔴), or omit for no icon"),
		),
	), handleRenameWindow)

	s.AddTool(mcp.NewTool("rename_pane",
		mcp.WithDescription("Set a visible title on a tmux pane, shown in the pane border. Useful for identifying agents by name."),
		mcp.WithString("target",
			mcp.Required(),
			mcp.Description("Pane target in session:window.pane format"),
		),
		mcp.WithString("title",
			mcp.Required(),
			mcp.Description("Title to display on the pane border, e.g. 'executor', 'reviewer', 'shell'"),
		),
	), handleRenamePane)

	s.AddTool(mcp.NewTool("create_pane",
		mcp.WithDescription("Create a new tmux pane and optionally start an agent in it. Returns the target (session:window.pane) of the new pane."),
		mcp.WithString("session",
			mcp.Required(),
			mcp.Description("Tmux session name where the pane will be created"),
		),
		mcp.WithString("command",
			mcp.Description("Command to run in the new pane, e.g. 'claude', 'codex', 'cursor'"),
		),
		mcp.WithString("name",
			mcp.Description("Window name (only used when creating a new window, not when splitting)"),
		),
		mcp.WithString("dir",
			mcp.Description("Working directory for the new pane"),
		),
		mcp.WithString("split",
			mcp.Description("Split the current window instead of creating a new one: 'h' (horizontal) or 'v' (vertical)"),
		),
	), handleCreatePane)

	s.AddTool(mcp.NewTool("wait_for_idle",
		mcp.WithDescription("Wait until an agent in a pane finishes its task and is ready for new input. Updates the window tab icon automatically: busy (⚡) while waiting, done (🟢) when idle, wait (🔴) on timeout or error."),
		mcp.WithString("target",
			mcp.Required(),
			mcp.Description("Pane target in session:window.pane format"),
		),
		mcp.WithNumber("timeout",
			mcp.Description("Maximum seconds to wait (default 300)"),
		),
		mcp.WithString("agent",
			mcp.Description("Agent type: claude, codex, cursor, shell, or auto (default auto)"),
		),
		mcp.WithBoolean("update_icon",
			mcp.Description("Update the window tab icon during wait (default true)"),
		),
	), handleWaitForIdle)

	if err := server.ServeStdio(s); err != nil {
		log.Fatal(err)
	}
}

func handleRenameWindow(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	target, err := req.RequireString("target")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	name, err := req.RequireString("name")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	status := req.GetString("status", "")

	if err := tmux.RenameWindow(target, name, status); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("window %s renamed", target)), nil
}

func handleRenamePane(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	target, err := req.RequireString("target")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	title, err := req.RequireString("title")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	if err := tmux.RenamePane(target, title); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("%s renamed to %q", target, title)), nil
}

func handleCreatePane(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	session, err := req.RequireString("session")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	name := req.GetString("name", "")
	dir := req.GetString("dir", "")
	command := req.GetString("command", "")
	split := req.GetString("split", "")

	target, err := tmux.CreatePane(session, name, dir, command, split)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("created pane %s", target)), nil
}

func handleListPanes(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	panes, err := tmux.ListPanes()
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	if len(panes) == 0 {
		return mcp.NewToolResultText("no panes found"), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%-30s %-12s %-10s %-10s %s\n", "TARGET", "SESSION", "WINDOW", "PANE", "COMMAND"))
	sb.WriteString(strings.Repeat("-", 80) + "\n")
	for _, p := range panes {
		sb.WriteString(fmt.Sprintf("%-30s %-12s %-10s %-10s %s\n",
			p.Target, p.Session, p.Window, p.Index, p.Command))
	}
	return mcp.NewToolResultText(sb.String()), nil
}

func handleSendToPane(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	target, err := req.RequireString("target")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	text, err := req.RequireString("text")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	enter := req.GetBool("enter", true)

	if err := tmux.SendToPane(target, text, enter); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("sent to %s", target)), nil
}

func handleReadPane(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	target, err := req.RequireString("target")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	lines := req.GetInt("lines", 50)

	output, err := tmux.ReadPane(target, lines)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(output), nil
}

func handleBroadcast(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	text, err := req.RequireString("text")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	enter := req.GetBool("enter", true)
	filter := req.GetString("filter", "all")

	count, err := tmux.Broadcast(text, enter, filter)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("sent to %d panes", count)), nil
}

func handleWaitForIdle(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	target, err := req.RequireString("target")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	timeout := req.GetInt("timeout", 300)
	agentType := req.GetString("agent", "auto")
	updateIcon := req.GetBool("update_icon", true)

	elapsed, err := tmux.WaitForIdle(target, agentType, timeout, updateIcon)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("idle after %s", elapsed)), nil
}
