package app

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

// Action represents a user choice from the interactive menu.
type Action int

const (
	ActionNone Action = iota
	ActionApply
	ActionSave
	ActionView
	ActionKeep
	ActionDiscard
	ActionQuit
)

// Prompter reads single-line responses from the user.
type Prompter struct {
	reader *bufio.Reader
	out    io.Writer
}

// NewPrompter wraps stdin/stderr.
func NewPrompter(in io.Reader, out io.Writer) *Prompter {
	return &Prompter{reader: bufio.NewReader(in), out: out}
}

// ReadLine reads a single trimmed line. Returns "" on EOF.
func (p *Prompter) ReadLine() (string, error) {
	line, err := p.reader.ReadString('\n')
	line = strings.TrimRight(line, "\r\n")
	if err == io.EOF {
		return line, nil
	}
	return line, err
}

// Confirm asks a yes/no question.
func (p *Prompter) Confirm(msg string, defaultYes bool) (bool, error) {
	suffix := " [y/N]: "
	if defaultYes {
		suffix = " [Y/n]: "
	}
	fmt.Fprint(p.out, msg, suffix)
	line, err := p.ReadLine()
	if err != nil {
		return false, err
	}
	line = strings.TrimSpace(strings.ToLower(line))
	if line == "" {
		return defaultYes, nil
	}
	return line == "y" || line == "yes", nil
}

// AskMenu prints the action menu and returns the chosen Action.
func (p *Prompter) AskMenu(defaultAction Action) (Action, error) {
	fmt.Fprintln(p.out, "Actions:")
	fmt.Fprintln(p.out, "  [a] apply patch")
	fmt.Fprintln(p.out, "  [s] save patch")
	fmt.Fprintln(p.out, "  [v] view patch")
	fmt.Fprintln(p.out, "  [k] keep worktree")
	fmt.Fprintln(p.out, "  [d] discard")
	prompt := "Choice"
	if def := defaultLetter(defaultAction); def != "" {
		prompt += fmt.Sprintf(" [%s]", def)
	}
	fmt.Fprint(p.out, prompt+": ")
	line, err := p.ReadLine()
	if err != nil {
		return ActionNone, err
	}
	choice := strings.TrimSpace(strings.ToLower(line))
	if choice == "" {
		return defaultAction, nil
	}
	switch choice {
	case "a", "apply":
		return ActionApply, nil
	case "s", "save":
		return ActionSave, nil
	case "v", "view":
		return ActionView, nil
	case "k", "keep":
		return ActionKeep, nil
	case "d", "discard":
		return ActionDiscard, nil
	case "q", "quit":
		return ActionQuit, nil
	}
	fmt.Fprintf(p.out, "Unknown choice %q.\n", choice)
	return ActionNone, nil
}

// AskPath prompts the user for a save path; returns default if blank.
func (p *Prompter) AskPath(defaultPath string) (string, error) {
	fmt.Fprintf(p.out, "Save patch to [%s]: ", defaultPath)
	line, err := p.ReadLine()
	if err != nil {
		return "", err
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return defaultPath, nil
	}
	return line, nil
}

// StdinIsTTY reports whether stdin appears to be an interactive terminal.
func StdinIsTTY() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

func defaultLetter(a Action) string {
	switch a {
	case ActionApply:
		return "a"
	case ActionSave:
		return "s"
	case ActionView:
		return "v"
	case ActionKeep:
		return "k"
	case ActionDiscard:
		return "d"
	}
	return ""
}
