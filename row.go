package main

// Agent is the kind of agent running in a tab.
type Agent string

const (
	AgentClaude Agent = "claude"
	AgentCodex  Agent = "codex"
)

var validAgents = []Agent{AgentClaude, AgentCodex}

// State is the state of an agent session in a tab, as shown in the state
// field. Claude Code reports the first four in its session file; a tab whose
// session no longer runs is stopped.
type State string

const (
	StateBusy    State = "busy"
	StateWaiting State = "waiting"
	StateIdle    State = "idle"
	StateShell   State = "shell"
	StateStopped State = "stopped"
	// StateUnknown is for a session whose state could not be determined
	// (Claude did not report it as a string, or Codex process data is unknown).
	StateUnknown State = "unknown"
)

// Row is one listed surface. Its JSON and table forms come from fields; the
// exported names follow the JSON keys.
type Row struct {
	Agent          Agent
	WindowRef      *string
	WorkspaceRef   *string
	WorkspaceTitle *string
	SurfaceRef     *string
	SurfaceTitle   *string
	State          State
	WaitingFor     WaitingFor // nil unless the session is waiting
	Updated        *int64
	Cwd            *string
	SessionID      string
	Pid            *int // nil when claude is not running
	TranscriptPath *string
	surfaceID      string // surface UUID from `cmux tree`; binds a Codex process to its tab

	wsID    string // workspace UUID from `cmux tree`
	wsIndex int    // workspace index in its window, -1 when unknown
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
