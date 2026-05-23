# Proposal: Adapters de status por agente

## Contexto

Hoje o `tmux-mcp` infere estado do agente de forma **indireta**:

- `wait_for_idle` faz poll de `capture-pane` e procura padrões na última linha (`❯`, `>`, `$`)
- ícones da tab são atualizados no início/fim do wait (⚡ → 🟢 ou 🔴)

Isso é leve e funciona como sinal visual aproximado, mas **não monitora o estado real do agente**. Limitações conhecidas:

- agente "pensando" sem mudar o prompt → falso `done`
- spinner/progress bar → falso `busy` ou `done` prematuro
- prompt visível mas tarefa incompleta → falso positivo
- cada CLI tem semântica diferente de idle, erro e conclusão

Para evolução futura, propomos **adapters por agente**: cada CLI expõe sinais nativos de execução; o tmux-mcp traduz isso em estado unificado e reflete na tab.

## Objetivo

Introduzir uma camada `AgentAdapter` que substitua (ou complemente) a heurística de pane output por detecção **assertiva por CLI**, mantendo tmux como camada de display/orquestração.

## Modelo de estado unificado

Estados canônicos do MCP (independente do agente):

| Estado | Ícone | Significado |
|--------|-------|-------------|
| `idle` | (sem prefixo ou 🔴) | Agente ocioso, pronto para input |
| `running` | ⚡ | Executando tarefa |
| `done` | 🟢 | Tarefa concluída com sucesso |
| `waiting` | 🔴 | Aguardando input do usuário ou bloqueado |
| `error` | 🔴 ou ❌ | Falha, timeout ou exit code != 0 |

Transições típicas:

```
idle → running → done → idle
idle → running → waiting → idle
idle → running → error
```

## Interface proposta (esboço)

```go
// internal/agent/adapter.go

type AgentState string

const (
    StateIdle    AgentState = "idle"
    StateRunning AgentState = "running"
    StateDone    AgentState = "done"
    StateWaiting AgentState = "waiting"
    StateError   AgentState = "error"
)

type StatusEvent struct {
    State     AgentState
    Message   string    // opcional: "build failed", "waiting for approval"
    Timestamp time.Time
}

type AgentAdapter interface {
    // Tipo de agente suportado: claude, codex, cursor, shell
    Type() string

    // Detecta se este adapter é aplicável ao pane (comando em execução)
    Supports(pane PaneInfo) bool

    // Snapshot do estado atual (não bloqueante)
    Poll(ctx context.Context, target string) (StatusEvent, error)

    // Aguarda transição para idle/done/error (substitui wait_for_idle genérico)
    Wait(ctx context.Context, target string, timeout time.Duration) (StatusEvent, error)
}
```

Registry:

```go
var adapters = []AgentAdapter{
    &ClaudeAdapter{},
    &CodexAdapter{},
    &CursorAdapter{},
    &ShellAdapter{}, // fallback atual (heurística de prompt)
}

func ResolveAdapter(pane PaneInfo) AgentAdapter { ... }
```

## Adapters por agente (esboço)

### ClaudeAdapter

Sinais nativos possíveis:

- modo non-interactive: `claude -p "..." --output-format json` → exit code + JSON estruturado
- hooks do Claude Code (`PreToolUse`, `PostToolUse`, `Stop`) emitindo status
- detecção de processo filho + exit code quando comando termina

Estratégia inicial (fase 1):

- manter pane interativo para UX
- detectar padrões específicos do Claude no output (melhor que shell genérico)
- evoluir para hooks/events quando disponível

Estratégia futura (fase 2):

- wrapper `claude -p` com parse de JSON de resposta
- estado `running` enquanto processo ativo; `done`/`error` no exit

### CodexAdapter

Sinais nativos possíveis:

- `codex exec` com exit code determinístico
- output estruturado / modo batch
- diff de output + detecção de "executing" vs prompt

Estratégia inicial:

- padrões de prompt específicos do Codex
- monitorar PID do processo no pane

Estratégia futura:

- preferir invocação via `codex exec` quando orquestrador controla o fluxo

### CursorAdapter

Sinais nativos possíveis:

- `cursor agent` / Cursor CLI com streaming e exit code
- SDK `@cursor/sdk` para runs com status (`running`, `completed`, `failed`)
- eventos de run quando agente roda fora do tmux

Estratégia inicial:

- heurística melhorada no pane (similar ao shell, com patterns Cursor)
- detecção via `pane_current_command` contendo `cursor`

Estratégia futura:

- integração com Cursor SDK para runs programáticos (mais assertivo que tmux)

### ShellAdapter (fallback)

Comportamento atual de `wait_for_idle` + `DetectAgent`:

- poll `capture-pane` + regex de prompt
- último recurso quando nenhum adapter específico se aplica

## Integração com tools existentes

| Tool | Comportamento futuro |
|------|---------------------|
| `wait_for_idle` | Delega para `ResolveAdapter(pane).Wait()`; fallback ShellAdapter |
| `rename_window` | Continua manual; adapters podem chamar internamente |
| `list_panes` | Opcional: coluna `STATE` via `Poll()` |
| novo: `get_agent_status` | Snapshot sem bloquear (útil para orquestrador) |

Fluxo orquestrador (evoluído):

```
send_to_pane     → adapter detecta running (⚡)
wait_for_idle    → adapter.Wait() → done/error (🟢/🔴)
read_pane        → resultado
get_agent_status → consulta pontual sem wait
```

## Fases de evolução sugeridas

### Fase 1 — Interface + fallback (baixo risco)

- criar pacote `internal/agent` com interface e registry
- migrar lógica atual de `detect.go` para `ShellAdapter`
- `wait_for_idle` usa registry; comportamento idêntico ao atual

### Fase 2 — Adapters heurísticos por CLI

- `ClaudeAdapter`, `CodexAdapter`, `CursorAdapter` com patterns específicos
- melhorar precisão sem mudar invocação dos agentes

### Fase 3 — Sinais nativos (assertivo)

- exit codes, JSON output, hooks, Cursor SDK
- tmux vira display layer; estado vem do adapter nativo

## Non-goals (desta change)

- Implementação dos adapters (apenas proposta)
- Watcher em background contínuo
- Event bus / sidecar externo (redis, sqlite)
- Substituir tmux como mecanismo de orquestração
- Breaking changes nas tools MCP existentes

## Critérios de sucesso (quando implementarmos)

- `wait_for_idle` com Claude/Codex/Cursor tem menos falsos positivos que ShellAdapter
- orquestrador não precisa chamar `rename_window` manualmente para sinalização básica
- fallback ShellAdapter preserva comportamento atual
- cada adapter testável isoladamente (mock de pane output / exit code)

## Referências

- `internal/tmux/detect.go` — heurística atual
- `internal/tmux/client.go` — `SetWindowStatus`, ícones de tab
- OpenSpec change: `agent-status-adapters` (este documento)
