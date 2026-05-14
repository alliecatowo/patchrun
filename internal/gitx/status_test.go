package gitx

import (
	"testing"
)

func TestParsePorcelainZ_Simple(t *testing.T) {
	data := []byte(" M a.txt\x00?? new.txt\x00A  staged.txt\x00")
	entries, err := parsePorcelainZ(data)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("want 3, got %d (%#v)", len(entries), entries)
	}
	if entries[0].WorkTreeStatus != 'M' || entries[0].Path != "a.txt" {
		t.Fatalf("modified: %#v", entries[0])
	}
	if !entries[1].IsUntracked() || entries[1].Path != "new.txt" {
		t.Fatalf("untracked: %#v", entries[1])
	}
	if entries[2].IndexStatus != 'A' || entries[2].Path != "staged.txt" {
		t.Fatalf("staged: %#v", entries[2])
	}
}

func TestParsePorcelainZ_Rename(t *testing.T) {
	data := []byte("R  new\x00old\x00 M other\x00")
	entries, err := parsePorcelainZ(data)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("want 2, got %d (%#v)", len(entries), entries)
	}
	if entries[0].IndexStatus != 'R' || entries[0].Path != "new" || entries[0].OldPath != "old" {
		t.Fatalf("rename: %#v", entries[0])
	}
	if entries[1].Path != "other" {
		t.Fatalf("plain: %#v", entries[1])
	}
}

func TestStatusFingerprint_Stable(t *testing.T) {
	e := []StatusEntry{
		{IndexStatus: ' ', WorkTreeStatus: 'M', Path: "a.txt"},
		{IndexStatus: '?', WorkTreeStatus: '?', Path: "b.txt"},
	}
	a := StatusFingerprint(e)
	b := StatusFingerprint(e)
	if a != b {
		t.Fatalf("fingerprint not stable")
	}
	e2 := []StatusEntry{
		{IndexStatus: ' ', WorkTreeStatus: 'M', Path: "a.txt"},
	}
	if StatusFingerprint(e2) == a {
		t.Fatalf("fingerprint should differ across inputs")
	}
}

func TestPatchIsEmpty(t *testing.T) {
	if !PatchIsEmpty(nil) {
		t.Fatalf("nil should be empty")
	}
	if !PatchIsEmpty([]byte("   \n\n")) {
		t.Fatalf("whitespace should be empty")
	}
	if PatchIsEmpty([]byte("diff --git a/x b/x\n")) {
		t.Fatalf("real diff should not be empty")
	}
}
