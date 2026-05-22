# tmux-mcp

MCP server that exposes tmux panes as tools, enabling AI agents (Claude Code, Codex, Cursor CLI) to communicate with each other across panes — and allowing an orchestrator agent to create, name, control, and read from any pane autonomously.

## Tools

| Tool | Description |
|---|---|
| `list_panes` | List all tmux panes with target, session, window, pane index and running command |
| `create_pane` | Create a new pane (new window or split) and optionally start an agent or shell in it |
| `rename_pane` | Set a visible title on a pane, shown in the pane border |
| `send_to_pane` | Send text or a command to a specific pane |
| `read_pane` | Read the current output of a pane |
| `broadcast` | Send text to multiple panes, optionally filtered by agent type |
| `wait_for_idle` | Wait until an agent finishes its task and is ready for new input |

## How it works

The MCP server wraps tmux's CLI. Each tool maps to one or more tmux commands:

| Tool | tmux command |
|---|---|
| `list_panes` | `tmux list-panes -a` |
| `create_pane` | `tmux new-window` or `tmux split-window` |
| `rename_pane` | `tmux select-pane -T` |
| `send_to_pane` | `tmux send-keys -t <target>` |
| `read_pane` | `tmux capture-pane -t <target> -p` |
| `broadcast` | `tmux send-keys` loop across panes |
| `wait_for_idle` | polling `capture-pane` until idle prompt detected |

## Pane targets

Format: `session:window.pane`

- `main:0.0` — session "main", window 0, pane 0
- `auth-service:1.0` — session "auth-service", window 1, pane 0

Use `list_panes` to discover available targets.

## Usage examples

### Shell command in a new pane

> *"Abra um terminal com shell e rode `npm test` na pasta ~/projetos/auth"*

```
create_pane  session=main  name=test-runner  dir=~/projetos/auth  command="npm test"
→ criou main:1.0

read_pane  target=main:1.0  lines=50
→ retorna o output do teste
```

### Validate something in a specific pane

> *"No terminal 'executor', valide se o build passou"*

```
list_panes
→ main:0.0 [claude] executor
→ main:0.1 [codex]  executor

send_to_pane  target=main:0.1  text="go build ./..."
wait_for_idle  target=main:0.1
read_pane  target=main:0.1  lines=20
```

### Full autonomous multi-agent flow

> *"Crie um executor com Codex e um revisor com Claude, implemente o middleware de auth e revise"*

```
create_pane  session=auth-service  name=executor  command=codex   dir=~/projetos/auth
→ auth-service:1.0
rename_pane  target=auth-service:1.0  title=executor

create_pane  session=auth-service  name=reviewer  command=claude  dir=~/projetos/auth  split=h
→ auth-service:1.1
rename_pane  target=auth-service:1.1  title=reviewer

send_to_pane  target=auth-service:1.0  text="implemente src/middleware/auth.go"
wait_for_idle  target=auth-service:1.0  timeout=600  agent=codex
read_pane     target=auth-service:1.0  lines=30

send_to_pane  target=auth-service:1.1  text="revise o arquivo src/middleware/auth.go"
wait_for_idle  target=auth-service:1.1  timeout=300  agent=claude
read_pane     target=auth-service:1.1  lines=30
```

## Visible pane titles

Add to `~/.tmux.conf` to show pane titles in the border:

```
set -g pane-border-status top
set -g pane-border-format " #{pane_index}: #{pane_title} "
```

After `rename_pane`, each pane shows its title in the border — making it easy to identify which agent is running where.

## Build

```bash
go build -o tmux-mcp .
```

## Install

```bash
cp tmux-mcp ~/.local/bin/
```

## Configure Claude Code

```bash
claude mcp add -s user tmux-mcp ~/.local/bin/tmux-mcp
```
