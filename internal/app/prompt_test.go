package app

import (
	"bytes"
	"strings"
	"testing"
)

func TestPrompter_Confirm_DefaultNo(t *testing.T) {
	cases := []struct {
		in       string
		want     bool
		defaultY bool
	}{
		{"\n", false, false},
		{"y\n", true, false},
		{"Y\n", true, false},
		{"yes\n", true, false},
		{"n\n", false, false},
		{"\n", true, true},
		{"n\n", false, true},
	}
	for _, c := range cases {
		var out bytes.Buffer
		p := NewPrompter(strings.NewReader(c.in), &out)
		got, err := p.Confirm("ok?", c.defaultY)
		if err != nil {
			t.Fatalf("confirm err: %v", err)
		}
		if got != c.want {
			t.Fatalf("input %q defaultY=%v: got %v want %v", c.in, c.defaultY, got, c.want)
		}
	}
}

func TestPrompter_AskMenu(t *testing.T) {
	cases := []struct {
		in   string
		want Action
	}{
		{"a\n", ActionApply},
		{"apply\n", ActionApply},
		{"s\n", ActionSave},
		{"v\n", ActionView},
		{"k\n", ActionKeep},
		{"d\n", ActionDiscard},
		{"q\n", ActionQuit},
		{"\n", ActionView}, // default
	}
	for _, c := range cases {
		var out bytes.Buffer
		p := NewPrompter(strings.NewReader(c.in), &out)
		got, err := p.AskMenu(ActionView)
		if err != nil {
			t.Fatalf("menu err: %v", err)
		}
		if got != c.want {
			t.Fatalf("input %q: got %v want %v", c.in, got, c.want)
		}
	}
}

func TestPrompter_AskPath_Default(t *testing.T) {
	var out bytes.Buffer
	p := NewPrompter(strings.NewReader("\n"), &out)
	got, err := p.AskPath("/default.patch")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got != "/default.patch" {
		t.Fatalf("got %q", got)
	}
}

func TestPrompter_AskPath_Override(t *testing.T) {
	var out bytes.Buffer
	p := NewPrompter(strings.NewReader("/custom.patch\n"), &out)
	got, err := p.AskPath("/default.patch")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got != "/custom.patch" {
		t.Fatalf("got %q", got)
	}
}

func TestPrompter_AskMenu_Unknown(t *testing.T) {
	var out bytes.Buffer
	p := NewPrompter(strings.NewReader("zzz\n"), &out)
	got, _ := p.AskMenu(ActionView)
	if got != ActionNone {
		t.Fatalf("unknown should return ActionNone, got %v", got)
	}
	if !strings.Contains(out.String(), "Unknown") {
		t.Fatalf("expected unknown message, got: %s", out.String())
	}
}
