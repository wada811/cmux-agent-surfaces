package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseCodexProcesses(t *testing.T) {
	const sid = "01A1A452-9753-7781-A14F-C6C6A257B19B"
	cmd := `--disable computer_use -c mcp_servers.cmux-cua.command="/Users/me/Library/Application Support/cmux/codex-cua" -c check_for_update_on_startup=false resume ` + sid + ` --no-daemon`
	out := fmt.Sprintf(`100 1 ?? /Users/me/.npm/bin/codex app-server daemon pid-update-loop
101 1 ?? /Users/me/.npm/bin/codex app-server --listen localhost:1234
102 1 ?? /Users/me/.npm/bin/codex-code-mode-host
103 1 ?? /Users/me/Library/Application Support/cmux-cua serve --socket x
104 1 ttys012 node /Users/me/.npm/bin/codex %s
105 104 ttys012 /Users/me/.npm/vendor/aarch64-apple-darwin/bin/codex %s
106 105 ttys012 /Users/me/Library/Application Support/cmux-cua mcp --socket x --codex-computer-use-compat
107 1 ttys013 node /Users/me/.npm/bin/codex --no-daemon
108 107 ttys013 /Users/me/.npm/vendor/aarch64-apple-darwin/bin/codex --no-daemon
109 1 ttys014 /bin/sh -c 'codex resume %s'
110 1 ttys014 grep codex resume %s
111 1 ttys014 claude --resume %s
broken row

`, cmd, cmd, sid, sid, sid)
	got := parseCodexProcesses([]byte(out))
	if len(got) != 4 {
		t.Fatalf("got %d processes: %+v", len(got), got)
	}
	if got[0].pid != 104 || got[1].pid != 105 || got[2].pid != 107 || got[3].pid != 108 {
		t.Errorf("unexpected pids: %+v", got)
	}
	if got[0].sid != strings.ToLower(sid) || got[2].sid != "" {
		t.Errorf("unexpected session ids: %+v", got)
	}
}

func TestParseCodexProcessesLongUnrelatedLine(t *testing.T) {
	longArg := strings.Repeat("x", 70_000)
	out := []byte("200 1 ?? unrelated " + longArg + "\n" +
		"201 1 ?? /Users/me/.npm/bin/codex resume 00000000-0000-4000-8000-000000000012 --no-daemon\n")

	got := parseCodexProcesses(out)
	if len(got) != 1 || got[0].pid != 201 || got[0].sid != "00000000-0000-4000-8000-000000000012" {
		t.Fatalf("long unrelated line hid the Codex process: %+v", got)
	}
}

func TestCodexSessionID(t *testing.T) {
	const id = "01a1a452-9753-7781-a14f-c6c6a257b19b"
	for _, c := range []struct{ command, want string }{
		{"codex resume " + strings.ToUpper(id), id},
		{"codex --long-flag x resume " + id + " --no-daemon", id},
		{"codex resume --last", ""},
		{"codex resume", ""},
		{"codex resume " + id + "x", ""},
		{"codex -c x=resume " + id, ""},
	} {
		if got := codexSessionID(c.command); got != c.want {
			t.Errorf("codexSessionID(%q) = %q, want %q", c.command, got, c.want)
		}
	}
}

func TestIsCodexTUI(t *testing.T) {
	for _, c := range []struct {
		command string
		want    bool
	}{
		{"codex", true},
		{"/a/b/codex resume 00000000-0000-4000-8000-000000000000", true},
		{"node /a/codex", true},
		{"codex-code-mode-host", false},
		{"/a/codex app-server", false},
		{"/Users/me/Library/Application Support/cmux-cua mcp --socket x", false},
		{"/usr/bin/nodecodex", false},
	} {
		if got := isCodexTUI(c.command); got != c.want {
			t.Errorf("isCodexTUI(%q) = %v, want %v", c.command, got, c.want)
		}
	}
}

func TestLiveCodex(t *testing.T) {
	procs := []codexProc{
		{pid: 10, ppid: 1, tty: "ttys012", sid: "ABC"},
		{pid: 11, ppid: 10, tty: "ttys012", sid: "ABC"},
		{pid: 20, ppid: 1, tty: "??", sid: ""},
	}
	got := liveCodex(procs)
	if !got.ok || got.bySid["abc"].pid != 10 || got.byTTY["ttys012"].pid != 10 {
		t.Errorf("wrong representatives: %+v", got)
	}
	if _, ok := got.byTTY["??"]; ok {
		t.Error("unknown tty was indexed")
	}
	if _, ok := got.bySid[""]; ok {
		t.Error("empty session id was indexed")
	}
}

func TestCodexHome(t *testing.T) {
	oldHome := home
	home = t.TempDir()
	t.Cleanup(func() { home = oldHome })
	t.Setenv("CODEX_HOME", "/tmp/codex-home")
	if got := codexHome(); got != "/tmp/codex-home" {
		t.Errorf("CODEX_HOME = %q", got)
	}
	t.Setenv("CODEX_HOME", "")
	if got, want := codexHome(), filepath.Join(home, ".codex"); got != want {
		t.Errorf("default codex home = %q, want %q", got, want)
	}
}

func TestCodexRollout(t *testing.T) {
	dir := t.TempDir()
	old := codexDir
	codexDir = dir
	t.Cleanup(func() { codexDir = old })
	sid := "00000000-0000-4000-8000-000000000012"
	paths := []string{
		filepath.Join(dir, "sessions", "2026", "10", "01", "rollout-a-"+sid+".jsonl"),
		filepath.Join(dir, "sessions", "2026", "10", "02", "rollout-b-"+sid+".jsonl"),
	}
	for _, p := range paths {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("{}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chtimes(paths[0], timeFromUnix(1), timeFromUnix(1)); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(paths[1], timeFromUnix(2), timeFromUnix(2)); err != nil {
		t.Fatal(err)
	}
	if got := codexRollout(sid); got == nil || *got != paths[1] {
		t.Errorf("codexRollout = %v, want %q", got, paths[1])
	}
}

func timeFromUnix(sec int64) time.Time { return time.Unix(sec, 0) }
