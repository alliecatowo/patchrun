package textui

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestColorizer_Disabled_Passthrough(t *testing.T) {
	c := NewColorizer(ColorNever, &bytes.Buffer{})
	if c.Enabled() {
		t.Fatalf("expected disabled")
	}
	for _, fn := range []func(string) string{c.Bold, c.Dim, c.Red, c.Green, c.Yellow, c.Blue, c.Magenta, c.Cyan} {
		if got := fn("x"); got != "x" {
			t.Fatalf("expected plain passthrough, got %q", got)
		}
	}
}

func TestColorizer_Always_WrapsEscapes(t *testing.T) {
	c := NewColorizer(ColorAlways, &bytes.Buffer{})
	if !c.Enabled() {
		t.Fatalf("expected enabled")
	}
	got := c.Red("hi")
	if !strings.HasPrefix(got, "\x1b[") || !strings.HasSuffix(got, "\x1b[0m") {
		t.Fatalf("expected ANSI-wrapped, got %q", got)
	}
}

func TestColorizer_NoColorEnv(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	c := NewColorizer(ColorAlways, &bytes.Buffer{})
	if c.Enabled() {
		t.Fatalf("NO_COLOR should disable color even with ColorAlways")
	}
}

func TestColorizer_AutoBuffer_DisabledByDefault(t *testing.T) {
	c := NewColorizer(ColorAuto, &bytes.Buffer{})
	if c.Enabled() {
		t.Fatalf("auto mode on a non-TTY writer should be disabled")
	}
}

func TestColorizer_AllMethods_Enabled(t *testing.T) {
	c := NewColorizer(ColorAlways, &bytes.Buffer{})
	wrappers := []struct {
		name string
		fn   func(string) string
	}{
		{"Bold", c.Bold},
		{"Dim", c.Dim},
		{"Red", c.Red},
		{"Green", c.Green},
		{"Yellow", c.Yellow},
		{"Blue", c.Blue},
		{"Magenta", c.Magenta},
		{"Cyan", c.Cyan},
	}
	for _, w := range wrappers {
		got := w.fn("x")
		if !strings.Contains(got, "\x1b[") || !strings.HasSuffix(got, "\x1b[0m") {
			t.Fatalf("%s: expected ANSI wrap, got %q", w.name, got)
		}
	}
}

func TestIsTerminal_VariousWriters(t *testing.T) {
	if isTerminal(nil) {
		t.Fatalf("nil should not be terminal")
	}
	if isTerminal(&bytes.Buffer{}) {
		t.Fatalf("buffer should not be terminal")
	}
	f, err := os.CreateTemp(t.TempDir(), "f")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if isTerminal(f) {
		t.Fatalf("regular file should not be terminal")
	}
}

func TestColorizer_AutoOnRegularFile(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "color-*")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	c := NewColorizer(ColorAuto, f)
	if c.Enabled() {
		t.Fatalf("regular file should not be detected as a TTY")
	}
}
