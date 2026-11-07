package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunFlags(t *testing.T) {
	for _, c := range []struct {
		name         string
		args         []string
		wantCode     int
		wantOut, err string
	}{
		{"help", []string{"--help"}, 0, "Options:", ""},
		{"-h", []string{"-h"}, 0, "Options:", ""},
		{"version", []string{"--version"}, 0, "cmux-agent-surfaces ", ""},
		{"unknown flag", []string{"--bogus"}, 2, "", "flag provided but not defined: -bogus"},
		{"positional", []string{"foo"}, 2, "", "unexpected argument: foo"},
	} {
		var out, errb bytes.Buffer
		code := run(c.args, &out, &errb)
		if code != c.wantCode || !strings.Contains(out.String(), c.wantOut) || !strings.Contains(errb.String(), c.err) {
			t.Errorf("%s: code=%d out=%q err=%q", c.name, code, out.String(), errb.String())
		}
		if c.wantCode == 2 && !strings.Contains(errb.String(), "usage: cmux-agent-surfaces") {
			t.Errorf("%s: no usage on stderr: %q", c.name, errb.String())
		}
	}
}

func TestRunUsageErrors(t *testing.T) {
	for _, args := range [][]string{
		{"--state", "foo"},
		{"--state", ""},
		{"--state", "waiting,foo"},
		{"--agent", "nope"},
		{"--older-than", "abc"},
		{"--older-than", "-1d"},
		{"--workspace", ""},
		{"--workspace", "  "},
		{"--sort", "foo"},
		{"--sort", ""},
	} {
		var out, errb bytes.Buffer
		code := run(args, &out, &errb)
		if code != 2 || out.Len() != 0 || !strings.Contains(errb.String(), "usage: cmux-agent-surfaces") ||
			!strings.Contains(errb.String(), "Run 'cmux-agent-surfaces --help' for details.") {
			t.Errorf("%v: code=%d out=%q err=%q", args, code, out.String(), errb.String())
		}
	}
}
