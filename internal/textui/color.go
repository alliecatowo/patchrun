package textui

import (
	"io"
	"os"
)

const (
	reset   = "\x1b[0m"
	bold    = "\x1b[1m"
	dim     = "\x1b[2m"
	red     = "\x1b[31m"
	green   = "\x1b[32m"
	yellow  = "\x1b[33m"
	blue    = "\x1b[34m"
	magenta = "\x1b[35m"
	cyan    = "\x1b[36m"
)

// ColorMode controls whether ANSI sequences are emitted.
type ColorMode int

const (
	ColorAuto ColorMode = iota
	ColorAlways
	ColorNever
)

// Colorizer wraps a writer with optional ANSI coloring.
type Colorizer struct {
	enabled bool
}

// NewColorizer chooses color emission based on mode and writer.
func NewColorizer(mode ColorMode, w io.Writer) *Colorizer {
	if os.Getenv("NO_COLOR") != "" {
		return &Colorizer{enabled: false}
	}
	switch mode {
	case ColorAlways:
		return &Colorizer{enabled: true}
	case ColorNever:
		return &Colorizer{enabled: false}
	default:
		return &Colorizer{enabled: isTerminal(w)}
	}
}

// Enabled reports whether the colorizer will emit escape codes.
func (c *Colorizer) Enabled() bool { return c.enabled }

func (c *Colorizer) wrap(code, s string) string {
	if !c.enabled {
		return s
	}
	return code + s + reset
}

func (c *Colorizer) Bold(s string) string    { return c.wrap(bold, s) }
func (c *Colorizer) Dim(s string) string     { return c.wrap(dim, s) }
func (c *Colorizer) Red(s string) string     { return c.wrap(red, s) }
func (c *Colorizer) Green(s string) string   { return c.wrap(green, s) }
func (c *Colorizer) Yellow(s string) string  { return c.wrap(yellow, s) }
func (c *Colorizer) Blue(s string) string    { return c.wrap(blue, s) }
func (c *Colorizer) Magenta(s string) string { return c.wrap(magenta, s) }
func (c *Colorizer) Cyan(s string) string    { return c.wrap(cyan, s) }

// isTerminal returns true if w is an *os.File attached to a terminal.
func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return fileIsTerminal(f)
}
