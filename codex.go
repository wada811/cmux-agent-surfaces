// Codex side: ~/.codex rollout transcripts and live processes.

package main

import (
	"bufio"
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

type codexProc struct {
	pid, ppid int
	tty, sid  string
}

var psLine = regexp.MustCompile(`^\s*(\d+)\s+(\d+)\s+(\S+)\s+(.*)$`)
var resumeID = regexp.MustCompile(`(^|\s)resume\s+([0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12})(\s|$)`)

func parseCodexProcesses(out []byte) []codexProc {
	var procs []codexProc
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		m := psLine.FindStringSubmatch(scanner.Text())
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

type codexLive struct {
	ok    bool
	bySid map[string]codexProc
	byTTY map[string]codexProc
}

func liveCodex(procs []codexProc) codexLive {
	live := codexLive{ok: true, bySid: map[string]codexProc{}, byTTY: map[string]codexProc{}}
	pids := map[int]bool{}
	for _, p := range procs {
		pids[p.pid] = true
	}
	for _, p := range procs {
		if pids[p.ppid] { // keep the wrapper, not its vendor child
			continue
		}
		if p.sid != "" {
			live.bySid[strings.ToLower(p.sid)] = p
		}
		if p.tty != "" && p.tty != "??" {
			live.byTTY[p.tty] = p
		}
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
	p, ok := live.bySid[strings.ToLower(rec.CheckpointID)]
	if !ok && row.tty != "" {
		p, ok = live.byTTY[row.tty]
	}
	if ok {
		row.State = StateUnknown
		row.Pid = &p.pid
		return
	}
	row.State = StateStopped
	row.Pid = nil
}
