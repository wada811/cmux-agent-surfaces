// Codex side: ~/.codex rollout transcripts and live processes.

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// codexDir is Codex's data directory. Tests replace it.
var codexDir = codexHome()

func codexHome() string {
	if dir := os.Getenv("CODEX_HOME"); dir != "" {
		return dir
	}
	return filepath.Join(home, ".codex")
}

// psExec reports full process command lines. The resume id is near the end of
// Codex's long command, so -ww is needed to avoid truncating it.
var psExec = func() ([]byte, error) {
	return exec.Command("ps", "-axww", "-o", "pid=,ppid=,tty=,command=").Output()
}

// psEnvExec reports the environment of the given pids (`ps eww`). ps exits 1
// when a pid ended midway but its output is still valid, so output and error
// are both returned. Tests replace it.
var psEnvExec = func(pids []int) ([]byte, error) {
	list := make([]string, len(pids))
	for i, p := range pids {
		list[i] = strconv.Itoa(p)
	}
	return exec.Command("ps", "eww", "-p", strings.Join(list, ","), "-o", "pid=,command=").Output()
}

type codexProc struct {
	pid, ppid int
	tty, sid  string
	surfaceID string // CMUX_SURFACE_ID of the process, upper case; "" until read
}

var psLine = regexp.MustCompile(`^\s*(\d+)\s+(\d+)\s+(\S+)\s+(.*)$`)
var resumeID = regexp.MustCompile(`(^|\s)resume\s+([0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12})(\s|$)`)

func parseCodexProcesses(out []byte) []codexProc {
	var procs []codexProc
	for _, line := range strings.Split(string(out), "\n") {
		m := psLine.FindStringSubmatch(line)
		if m == nil || !isCodexTUI(m[4]) {
			continue
		}
		pid, _ := strconv.Atoi(m[1])
		ppid, _ := strconv.Atoi(m[2])
		procs = append(procs, codexProc{pid: pid, ppid: ppid, tty: m[3], sid: codexSessionID(m[4])})
	}
	return procs
}

func isCodexTUI(command string) bool {
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return false
	}
	exe := fields[0]
	if filepath.Base(exe) == "node" {
		if len(fields) < 2 {
			return false
		}
		exe = fields[1]
	}
	if filepath.Base(exe) != "codex" {
		return false
	}
	// The resident processes keep a stale CMUX_SURFACE_ID.
	if strings.Contains(exe, "app-server-daemon") {
		return false
	}
	for _, arg := range fields {
		if arg == "app-server" {
			return false
		}
	}
	// Splitting on fields misses Codex executable paths that contain spaces.
	return true
}

func codexSessionID(command string) string {
	if m := resumeID.FindStringSubmatch(command); m != nil {
		return strings.ToLower(m[2])
	}
	return ""
}

var surfaceIDEnv = regexp.MustCompile(`CMUX_SURFACE_ID=([0-9A-Fa-f-]{36})`)
var psEnvLine = regexp.MustCompile(`^\s*(\d+)\s+(.*)$`)

// parseCodexSurfaceIDs reads CMUX_SURFACE_ID per pid from `ps eww -o pid=,command=`.
func parseCodexSurfaceIDs(out []byte) map[int]string {
	ids := map[int]string{}
	for _, line := range strings.Split(string(out), "\n") {
		m := psEnvLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		if e := surfaceIDEnv.FindStringSubmatch(m[2]); e != nil {
			pid, _ := strconv.Atoi(m[1])
			ids[pid] = strings.ToUpper(e[1])
		}
	}
	return ids
}

// readCodexSurfaceIDs fills surfaceID of procs from their environment. It
// reports false when the environment could not be read at all.
func readCodexSurfaceIDs(procs []codexProc) bool {
	if len(procs) == 0 {
		return true
	}
	pids := make([]int, len(procs))
	for i, p := range procs {
		pids[i] = p.pid
	}
	out, err := psEnvExec(pids)
	if err != nil && len(out) == 0 {
		return false
	}
	ids := parseCodexSurfaceIDs(out)
	for i := range procs {
		procs[i].surfaceID = ids[procs[i].pid]
	}
	return true
}

type codexLive struct {
	ok        bool
	bySid     map[string]codexProc
	bySurface map[string]codexProc // CMUX_SURFACE_ID (upper case) -> process
}

func liveCodex(procs []codexProc) codexLive {
	live := codexLive{ok: true, bySid: map[string]codexProc{}, bySurface: map[string]codexProc{}}
	byPid := map[int]codexProc{}
	for _, p := range procs {
		byPid[p.pid] = p
	}
	for _, p := range procs {
		if _, child := byPid[p.ppid]; child { // keep the wrapper, not its vendor child
			continue
		}
		if p.sid != "" {
			live.bySid[strings.ToLower(p.sid)] = p
		}
	}
	// The wrapper or its vendor child may be the one that carries the id.
	for _, p := range procs {
		if p.surfaceID == "" {
			continue
		}
		root := p
		for i := 0; i < len(procs); i++ {
			parent, ok := byPid[root.ppid]
			if !ok {
				break
			}
			root = parent
		}
		live.bySurface[p.surfaceID] = root
	}
	return live
}

func codexRollout(sid string) *string {
	paths, _ := filepath.Glob(filepath.Join(codexDir, "sessions", "*", "*", "*", "rollout-*-"+sid+".jsonl"))
	var best string
	var bestTime time.Time
	for _, path := range paths {
		st, err := os.Stat(path)
		if err != nil {
			continue
		}
		if best == "" || st.ModTime().After(bestTime) {
			best, bestTime = path, st.ModTime()
		}
	}
	if best == "" {
		return nil
	}
	return &best
}

func describeCodex(row *Row, rec *restoreRecord, live codexLive) {
	row.Agent = AgentCodex
	row.WaitingFor = nil
	row.Updated = nil
	row.SessionID = rec.CheckpointID
	row.TranscriptPath = codexRollout(rec.CheckpointID)
	if row.TranscriptPath != nil {
		ts := lastTimestamp(*row.TranscriptPath)
		row.Updated = &ts
	}
	if rec.LaunchCommand != nil && rec.LaunchCommand.WorkingDirectory != nil && *rec.LaunchCommand.WorkingDirectory != "" {
		row.Cwd = rec.LaunchCommand.WorkingDirectory
	}
	if !live.ok {
		row.State = StateUnknown
		row.Pid = nil
		return
	}
	// Bind by the CMUX_SURFACE_ID in the process environment. The tty in
	// `cmux tree` can be stale and point at another tab, so it is not used.
	var p codexProc
	ok := false
	if row.surfaceID != "" {
		p, ok = live.bySurface[strings.ToUpper(row.surfaceID)]
	}
	if !ok {
		p, ok = live.bySid[strings.ToLower(rec.CheckpointID)]
	}
	if ok {
		row.State = StateUnknown
		row.Pid = &p.pid
		return
	}
	row.State = StateStopped
	row.Pid = nil
}
