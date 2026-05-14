package textui

import (
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
)

// NameStatusEntry represents a single line of `git diff --name-status -z`.
type NameStatusEntry struct {
	Status  string // single letter: A M D R C T U
	Path    string
	OldPath string // populated for R/C
}

// NumstatEntry represents a single line of `git diff --numstat -z`.
type NumstatEntry struct {
	Insertions int
	Deletions  int
	Binary     bool
	Path       string
	OldPath    string
}

// Totals computed from numstat lines.
type Totals struct {
	Files      int
	Insertions int
	Deletions  int
	Binary     int
}

// ParseNameStatusZ parses output of `git diff --name-status -z`.
// Records are NUL-separated. R/C status entries occupy three NUL fields.
func ParseNameStatusZ(data []byte) ([]NameStatusEntry, error) {
	fields := splitZ(data)
	var out []NameStatusEntry
	for i := 0; i < len(fields); {
		raw := fields[i]
		if raw == "" {
			i++
			continue
		}
		statusLetter := string(raw[0])
		if statusLetter == "R" || statusLetter == "C" {
			if i+2 >= len(fields) {
				return nil, fmt.Errorf("malformed name-status near %q", raw)
			}
			out = append(out, NameStatusEntry{
				Status:  statusLetter,
				OldPath: fields[i+1],
				Path:    fields[i+2],
			})
			i += 3
			continue
		}
		if i+1 >= len(fields) {
			return nil, fmt.Errorf("malformed name-status near %q", raw)
		}
		out = append(out, NameStatusEntry{
			Status: statusLetter,
			Path:   fields[i+1],
		})
		i += 2
	}
	return out, nil
}

// ParseNumstatZ parses output of `git diff --numstat -z`.
// Format (non-rename): "<ins>\t<del>\t<path>\0"
// Format (rename):     "<ins>\t<del>\t\0<oldpath>\0<newpath>\0"
func ParseNumstatZ(data []byte) ([]NumstatEntry, error) {
	fields := splitZ(data)
	var out []NumstatEntry
	for i := 0; i < len(fields); {
		raw := fields[i]
		if raw == "" {
			i++
			continue
		}
		parts := strings.SplitN(raw, "\t", 3)
		if len(parts) < 3 {
			return nil, fmt.Errorf("malformed numstat line %q", raw)
		}
		entry := NumstatEntry{}
		if parts[0] == "-" && parts[1] == "-" {
			entry.Binary = true
		} else {
			ins, err := strconv.Atoi(parts[0])
			if err != nil {
				return nil, fmt.Errorf("bad insertions in %q", raw)
			}
			del, err := strconv.Atoi(parts[1])
			if err != nil {
				return nil, fmt.Errorf("bad deletions in %q", raw)
			}
			entry.Insertions = ins
			entry.Deletions = del
		}
		if parts[2] != "" {
			entry.Path = parts[2]
			out = append(out, entry)
			i++
			continue
		}
		// Rename/copy: two extra NUL-separated path fields follow.
		if i+2 >= len(fields) {
			return nil, fmt.Errorf("malformed rename numstat at index %d", i)
		}
		entry.OldPath = fields[i+1]
		entry.Path = fields[i+2]
		out = append(out, entry)
		i += 3
	}
	return out, nil
}

// Sum computes totals from numstat entries.
func Sum(entries []NumstatEntry) Totals {
	t := Totals{Files: len(entries)}
	for _, e := range entries {
		if e.Binary {
			t.Binary++
			continue
		}
		t.Insertions += e.Insertions
		t.Deletions += e.Deletions
	}
	return t
}

// FormatNameStatus renders a colored list of file changes.
func FormatNameStatus(c *Colorizer, entries []NameStatusEntry) string {
	sorted := make([]NameStatusEntry, len(entries))
	copy(sorted, entries)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Path < sorted[j].Path })

	var b strings.Builder
	for _, e := range sorted {
		statusCol := e.Status
		switch e.Status {
		case "A":
			statusCol = c.Green(e.Status)
		case "M":
			statusCol = c.Yellow(e.Status)
		case "D":
			statusCol = c.Red(e.Status)
		case "R", "C":
			statusCol = c.Blue(e.Status)
		case "T":
			statusCol = c.Magenta(e.Status)
		}
		if e.OldPath != "" {
			fmt.Fprintf(&b, "  %s %s -> %s\n", statusCol, e.OldPath, e.Path)
		} else {
			fmt.Fprintf(&b, "  %s %s\n", statusCol, e.Path)
		}
	}
	return b.String()
}

// FormatSummary renders the count summary line.
func FormatSummary(t Totals) string {
	insWord := "insertions"
	if t.Insertions == 1 {
		insWord = "insertion"
	}
	delWord := "deletions"
	if t.Deletions == 1 {
		delWord = "deletion"
	}
	fileWord := "files"
	if t.Files == 1 {
		fileWord = "file"
	}
	s := fmt.Sprintf("%d %s changed, %d %s, %d %s", t.Files, fileWord, t.Insertions, insWord, t.Deletions, delWord)
	if t.Binary > 0 {
		s += fmt.Sprintf(" (%d binary)", t.Binary)
	}
	return s
}

// WriteSummary writes a full human summary block.
func WriteSummary(w io.Writer, c *Colorizer, entries []NameStatusEntry, totals Totals, showStat bool) {
	if len(entries) == 0 {
		fmt.Fprintln(w, c.Dim("No repo changes."))
		return
	}
	plural := "files"
	if len(entries) == 1 {
		plural = "file"
	}
	fmt.Fprintf(w, "Changed %d %s:\n", len(entries), plural)
	fmt.Fprint(w, FormatNameStatus(c, entries))
	if showStat {
		fmt.Fprintf(w, "\nSummary:\n  %s\n", FormatSummary(totals))
	}
}

func splitZ(data []byte) []string {
	if len(data) == 0 {
		return nil
	}
	return strings.Split(strings.TrimSuffix(string(data), "\x00"), "\x00")
}
