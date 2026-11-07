package main

const usageLine = "usage: cmux-agent-surfaces [--agent LIST] [--state LIST] [--older-than DURATION] [--workspace WS] [--exclude-self] [--sort tree|state|age] [--fields LIST] [--json] [--completion SHELL] [--version]"

// helpText is written for both people and LLM agents: what the tool is for,
// what each value means and what to do next.
const helpText = `cmux-agent-surfaces - list the cmux surfaces (tabs) that run Claude Code or Codex.

When Claude Code and Codex sessions run in cmux tabs, this lists their tabs
across all windows and workspaces, with the agent and its state. By default,
both agents are listed.
Use it to:
  - find sessions waiting for your answer        (--state waiting)
  - find finished tabs that can be closed        (--state stopped --older-than 14d)
  - get a tab's session id and transcript path   (--json, or
                                                  --fields session_id,transcript_path)

` + usageLine + `

Options:
  --agent LIST           keep only these agents, comma-separated: claude,
                         codex (e.g. claude,codex)
  --state LIST           keep only these states, comma-separated, from busy,
                         waiting, idle, shell, unknown, stopped
                         (e.g. waiting,idle)
  --older-than DURATION  keep only tabs whose last message is older than
                         DURATION: 30m, 12h, 14d or any Go duration (1h30m).
                         Tabs with no known last message are left out.
  --workspace WS         keep only this workspace; takes the same values as
                         the --workspace flag of cmux commands (workspace:3).
                         An unknown value lists nothing and exits with 1.
  --exclude-self         leave out the tab this command runs in (found through
                         cmux's CMUX_SURFACE_ID; does nothing outside cmux)
  --sort ORDER           tree   cmux sidebar order (default)
                         state  waiting, idle, shell, busy, unknown, stopped;
                                newest first within a state
                         age    newest last message first
  --fields LIST          print only these fields, in this order,
                         comma-separated (e.g. surface_ref,state,age); see
                         Fields. Works for the table and for --json
  --json                 print the list as JSON, every field unless --fields
  --completion SHELL     print the shell completion script and exit; SHELL is
                         zsh (see Shell completion)
  --version              print the version and exit
  -h, --help             print this help

Filters, --sort and --fields apply to the table and to --json alike.

Fields:
  Each field is a JSON key and, in upper case, a table header. The table
  shows the ones marked * unless --fields is given; JSON shows them all.

  window_ref         cmux ref of the window
  workspace_ref *    cmux ref of the workspace
  workspace_title *  title of the workspace
  surface_ref *      cmux ref of the tab
  surface_title *    title of the tab
  agent *            claude or codex
  state *            busy     working on a reply
                     waiting  blocked on a question or a permission prompt
                     idle     reply finished; at the prompt
                     shell    reply finished; background shells or agents
                              still run
                     unknown  the agent's state could not be determined;
                              running Codex and a failed ps check use this
                     stopped  the agent has exited; the tab is still open
  waiting_for        what a waiting session is blocked on (see below); the
                     table shows one line such as "question: <header>"
                     Codex is always null and is never shown as waiting
  updated            time of the last message (JSON: Unix seconds; table:
                     local time)
  age *              time since the last message (JSON: seconds; table: 3h)
  cwd *              working directory (the table shortens $HOME to ~)
  session_id         Claude Code or Codex session id
  pid                process id when found, else null (table: -); for Codex,
                     this is the TUI process, usually the node wrapper
  transcript_path    Claude Code .jsonl or Codex rollout .jsonl transcript

  For Claude Code, these values match the inputs to its hooks.

waiting_for says what a "waiting" session is blocked on, read from the end of
its transcript. It is null for other states and when the transcript cannot be
read.
  {"kind": "question", "questions": [{"question": "...", "header": "...",
     "options": ["label", ...], "multi_select": false}]}
                         an AskUserQuestion is open
  {"kind": "permission", "tool": "Bash", "input": "<command or path>"}
                         a tool call awaits your approval; input is the
                         command (Bash), the file path (Edit, Write, Read) or
                         the tool input as one short line of JSON
  {"kind": "unknown"}    waiting, but no open question or tool call was found

Examples:
  cmux-agent-surfaces
  cmux-agent-surfaces --agent claude --state waiting
  cmux-agent-surfaces --agent codex
  cmux-agent-surfaces --state stopped --older-than 14d
  cmux-agent-surfaces --sort state
  cmux-agent-surfaces --exclude-self --sort state
  cmux-agent-surfaces --fields surface_ref,surface_title,state,age
  cmux-agent-surfaces --state waiting --fields surface_ref,waiting_for
  cmux-agent-surfaces --workspace workspace:3 --state waiting
  cmux-agent-surfaces --state waiting --json --fields surface_ref,waiting_for
  cmux-agent-surfaces --state waiting --json | jq -r '.[].transcript_path'
  cmux read-screen --surface surface:9   # look at a listed tab
  cmux close-surface --workspace <workspace_ref> --surface <surface_ref>
                                         # close one; take both refs from one row

Shell completion (zsh):
  Homebrew installs it. Otherwise put the script on your fpath, then run
  compinit, or load it in .zshrc:
    cmux-agent-surfaces --completion zsh > ~/.zfunc/_cmux-agent-surfaces
    source <(cmux-agent-surfaces --completion zsh)
  It completes the flags and the values of --agent, --state, --fields, --sort
  and --completion. --agent, --state and --fields take comma-separated lists.

Exit status (like grep):
  0  at least one row was printed
  1  no row to print: no agent tab, or the filters left none ("[]" with --json)
  2  error: bad usage, or cmux or its output could not be read
Warnings (e.g. a tab whose restore record could not be read) go to stderr and
do not change the exit status.

Claude and Codex tabs are identified by cmux's restore record. A Codex process
is bound to its tab by the CMUX_SURFACE_ID in its environment (read with ps eww),
falling back to the resume id in its command line. The tty in cmux tree is not
used, because it can be stale and point at another tab. If the environment
cannot be read, Codex state is unknown.`
