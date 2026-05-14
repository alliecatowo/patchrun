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
