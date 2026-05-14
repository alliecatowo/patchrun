package app

import (
	"bytes"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
)

func TestDefaultLetter_AllActions(t *testing.T) {
	cases := map[Action]string{
		ActionApply:   "a",
		ActionSave:    "s",
		ActionView:    "v",
		ActionKeep:    "k",
		ActionDiscard: "d",
		ActionNone:    "",
		ActionQuit:    "",
	}
	for a, want := range cases {
		if got := defaultLetter(a); got != want {
			t.Fatalf("defaultLetter(%v) = %q, want %q", a, got, want)
		}
	}
}

func TestReadLine_EOFWithData(t *testing.T) {
	var out bytes.Buffer
	p := NewPrompter(strings.NewReader("hello"), &out) // no trailing newline
	line, err := p.ReadLine()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if line != "hello" {
		t.Fatalf("got %q", line)
	}
}

func TestReadLine_EOFEmpty(t *testing.T) {
	var out bytes.Buffer
	p := NewPrompter(strings.NewReader(""), &out)
	_, err := p.ReadLine()
	if !errors.Is(err, io.EOF) {
		t.Fatalf("expected io.EOF, got %v", err)
	}
}

func TestStdinIsTTY_VariousReaders(t *testing.T) {
	if StdinIsTTY(strings.NewReader("")) {
		t.Fatalf("string reader should not be TTY")
	}
	if StdinIsTTY(nil) {
		t.Fatalf("nil should not be TTY")
	}
	// /dev/null is a char device on Linux; we accept that.
	f, err := os.Open(os.DevNull)
	if err != nil {
		t.Skip(err)
	}
	defer f.Close()
	_ = StdinIsTTY(f) // result depends on OS — just exercise the path
}

func TestAskMenu_AllAliases(t *testing.T) {
	cases := []struct {
		in   string
		want Action
	}{
		{"apply\n", ActionApply},
		{"save\n", ActionSave},
		{"view\n", ActionView},
		{"keep\n", ActionKeep},
		{"discard\n", ActionDiscard},
		{"quit\n", ActionQuit},
	}
	for _, c := range cases {
		var out bytes.Buffer
		p := NewPrompter(strings.NewReader(c.in), &out)
		got, err := p.AskMenu(ActionView)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if got != c.want {
			t.Fatalf("input %q got %v want %v", c.in, got, c.want)
		}
	}
}
