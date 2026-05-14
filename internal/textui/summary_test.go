package textui

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseNameStatusZ_Simple(t *testing.T) {
	data := []byte("M\x00a.txt\x00A\x00b.txt\x00D\x00c.txt\x00")
	got, err := ParseNameStatusZ(data)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	want := []NameStatusEntry{
		{Status: "M", Path: "a.txt"},
		{Status: "A", Path: "b.txt"},
		{Status: "D", Path: "c.txt"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
}

func TestParseNameStatusZ_Rename(t *testing.T) {
	data := []byte("R100\x00old.txt\x00new.txt\x00M\x00other.txt\x00")
	got, err := ParseNameStatusZ(data)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 entries, got %d (%#v)", len(got), got)
	}
	if got[0].Status != "R" || got[0].OldPath != "old.txt" || got[0].Path != "new.txt" {
		t.Fatalf("rename parse: %#v", got[0])
	}
	if got[1].Status != "M" || got[1].Path != "other.txt" {
		t.Fatalf("simple parse: %#v", got[1])
	}
}

func TestParseNumstatZ_Plain(t *testing.T) {
	// numstat -z record: "<ins>\t<del>\t<path>\0"
	data := []byte("3\t1\ta.txt\x00-\t-\tbin.dat\x00")
	got, err := ParseNumstatZ(data)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 entries, got %d (%#v)", len(got), got)
	}
	if got[0].Insertions != 3 || got[0].Deletions != 1 || got[0].Path != "a.txt" {
		t.Fatalf("first: %#v", got[0])
	}
	if !got[1].Binary || got[1].Path != "bin.dat" {
		t.Fatalf("binary: %#v", got[1])
	}
}

func TestParseNumstatZ_Rename(t *testing.T) {
	// rename: "<ins>\t<del>\t\0<oldpath>\0<newpath>\0"
	data := []byte("5\t2\t\x00old\x00new\x00")
	got, err := ParseNumstatZ(data)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 entry, got %d", len(got))
	}
	e := got[0]
	if e.Insertions != 5 || e.Deletions != 2 || e.OldPath != "old" || e.Path != "new" {
		t.Fatalf("rename: %#v", e)
	}
}

func TestFormatSummary_Pluralization(t *testing.T) {
	s := FormatSummary(Totals{Files: 1, Insertions: 1, Deletions: 1})
	if !strings.Contains(s, "1 file changed") {
		t.Fatalf("expected singular 'file', got: %s", s)
	}
	if !strings.Contains(s, "1 insertion") || strings.Contains(s, "1 insertions") {
		t.Fatalf("expected singular 'insertion', got: %s", s)
	}
	s2 := FormatSummary(Totals{Files: 2, Insertions: 3, Deletions: 0, Binary: 1})
	if !strings.Contains(s2, "2 files changed") {
		t.Fatalf("expected plural files, got: %s", s2)
	}
	if !strings.Contains(s2, "(1 binary)") {
		t.Fatalf("expected binary count, got: %s", s2)
	}
}
