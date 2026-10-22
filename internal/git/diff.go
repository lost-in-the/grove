package git

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// LineKind is what a diff line does to the file.
type LineKind int

const (
	LineContext   LineKind = iota // unchanged, on both sides
	LineAdded                     // only on the new side
	LineRemoved                   // only on the old side
	LineNoNewline                 // "\ No newline at end of file", on neither side
)

// DiffLine is one line of a hunk. OldNo and NewNo are its line numbers on
// each side, 0 on a side it is not on.
type DiffLine struct {
	Kind         LineKind
	Text         string
	OldNo, NewNo int
}

// Hunk is one @@ block of a file's diff.
type Hunk struct {
	Header             string // the whole @@ line
	OldStart, OldLines int
	NewStart, NewLines int
	Lines              []DiffLine
}

// FileStatus is how a diff changes a file.
type FileStatus int

const (
	FileModified FileStatus = iota
	FileAdded
	FileDeleted
	FileRenamed
)

// FileDiff is one file of a diff. OldPath is empty for an added file and
// NewPath for a deleted one. A binary or pure-rename file has no hunks.
type FileDiff struct {
	OldPath, NewPath string
	Status           FileStatus
	Binary           bool
	Hunks            []Hunk
	Added, Removed   int
}

// Path is the file's path: the new one, or the old one when deleted.
func (f FileDiff) Path() string {
	if f.NewPath != "" {
		return f.NewPath
	}
	return f.OldPath
}

var hunkHeader = regexp.MustCompile(`^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@(.*)$`)

// ParseDiff parses unified diff text, as from `git diff` or `gh pr diff`,
// into files, hunks, and numbered lines. Text before the first file is
// ignored. An empty diff gives nil, nil; a malformed one an error naming
// the line it is on.
func ParseDiff(diff string) ([]FileDiff, error) {
	if diff == "" {
		return nil, nil
	}
	lines := strings.Split(strings.TrimSuffix(diff, "\n"), "\n")
	var files []FileDiff
	var f *FileDiff
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if rest, ok := strings.CutPrefix(line, "diff --git "); ok {
			files = append(files, FileDiff{})
			f = &files[len(files)-1]
			f.OldPath, f.NewPath = headerPaths(rest)
			continue
		}
		if f == nil {
			continue
		}
		switch {
		case strings.HasPrefix(line, "@@"):
			h, next, err := parseHunk(lines, i)
			if err != nil {
				return nil, err
			}
			for _, l := range h.Lines {
				switch l.Kind {
				case LineAdded:
					f.Added++
				case LineRemoved:
					f.Removed++
				}
			}
			f.Hunks = append(f.Hunks, h)
			i = next - 1
		case strings.HasPrefix(line, "new file mode"):
			f.Status, f.OldPath = FileAdded, ""
		case strings.HasPrefix(line, "deleted file mode"):
			f.Status, f.NewPath = FileDeleted, ""
		case strings.HasPrefix(line, "rename from "):
			f.Status, f.OldPath = FileRenamed, unquotePath(strings.TrimPrefix(line, "rename from "))
		case strings.HasPrefix(line, "rename to "):
			f.NewPath = unquotePath(strings.TrimPrefix(line, "rename to "))
		case strings.HasPrefix(line, "--- "):
			f.OldPath = sidePath(strings.TrimPrefix(line, "--- "), "a/")
		case strings.HasPrefix(line, "+++ "):
			f.NewPath = sidePath(strings.TrimPrefix(line, "+++ "), "b/")
		case strings.HasPrefix(line, "Binary files "):
			f.Binary = true
		}
	}
	return files, nil
}

// parseHunk reads the hunk whose header is lines[i]. It ends by the
// header's line counts, never by a line's prefix, so content such as
// "--- x" stays a removed line. It returns the index after the hunk.
func parseHunk(lines []string, i int) (Hunk, int, error) {
	m := hunkHeader.FindStringSubmatch(lines[i])
	if m == nil {
		return Hunk{}, 0, fmt.Errorf("line %d: bad hunk header %q", i+1, lines[i])
	}
	h := Hunk{Header: lines[i], OldStart: atoi(m[1]), OldLines: count(m[2]), NewStart: atoi(m[3]), NewLines: count(m[4])}
	oldLeft, newLeft := h.OldLines, h.NewLines
	oldNo, newNo := h.OldStart, h.NewStart
	j := i + 1
	for ; oldLeft > 0 || newLeft > 0; j++ {
		if j >= len(lines) {
			return Hunk{}, 0, fmt.Errorf("line %d: hunk ends early", i+1)
		}
		l := lines[j]
		kind, text := LineContext, "" // a blank line is context whose space was stripped
		if l != "" {
			text = l[1:]
			switch l[0] {
			case ' ':
			case '+':
				kind = LineAdded
			case '-':
				kind = LineRemoved
			case '\\':
				kind = LineNoNewline
			default:
				return Hunk{}, 0, fmt.Errorf("line %d: not a diff line in the hunk at line %d", j+1, i+1)
			}
		}
		dl := DiffLine{Kind: kind, Text: text}
		switch kind {
		case LineContext:
			dl.OldNo, dl.NewNo = oldNo, newNo
			oldNo, newNo, oldLeft, newLeft = oldNo+1, newNo+1, oldLeft-1, newLeft-1
		case LineRemoved:
			dl.OldNo = oldNo
			oldNo, oldLeft = oldNo+1, oldLeft-1
		case LineAdded:
			dl.NewNo = newNo
			newNo, newLeft = newNo+1, newLeft-1
		case LineNoNewline:
			dl.Text = strings.TrimPrefix(text, " ")
		}
		if oldLeft < 0 || newLeft < 0 {
			return Hunk{}, 0, fmt.Errorf("line %d: hunk is longer than its header says", i+1)
		}
		h.Lines = append(h.Lines, dl)
	}
	// A no-newline marker can follow the hunk's last line.
	for ; j < len(lines) && strings.HasPrefix(lines[j], `\`); j++ {
		h.Lines = append(h.Lines, DiffLine{Kind: LineNoNewline, Text: strings.TrimPrefix(lines[j], `\ `)})
	}
	return h, j, nil
}

// headerPaths reads the two paths of a "diff --git" header, the only place
// a binary or mode-only change names its file. Unquoted, the paths are
// split where "a/P b/P" splits for one P; a rename's real paths come from
// its "rename from/to" lines.
func headerPaths(s string) (string, string) {
	if strings.HasPrefix(s, `"`) {
		if a, err := strconv.QuotedPrefix(s); err == nil {
			b := strings.TrimSpace(s[len(a):])
			return sidePath(a, "a/"), sidePath(b, "b/")
		}
	}
	if n := len(s) - 5; n > 0 && n%2 == 0 {
		if p := s[2 : 2+n/2]; s == "a/"+p+" b/"+p {
			return p, p
		}
	}
	if i := strings.LastIndex(s, " b/"); i >= 0 {
		return sidePath(s[:i], "a/"), sidePath(s[i+1:], "b/")
	}
	return "", ""
}

// sidePath is the path on a ---/+++ line: "" for /dev/null, unquoted, with
// its a/ or b/ prefix dropped. git ends a path holding a space with a tab,
// which is not part of the name.
func sidePath(raw, prefix string) string {
	raw = strings.TrimSuffix(raw, "\t")
	if raw == "/dev/null" {
		return ""
	}
	return strings.TrimPrefix(unquotePath(raw), prefix)
}

// unquotePath undoes git's C-style quoting of a path holding spaces,
// control characters, or (with core.quotePath) non-ASCII bytes.
func unquotePath(p string) string {
	if strings.HasPrefix(p, `"`) {
		if u, err := strconv.Unquote(p); err == nil {
			return u
		}
	}
	return p
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

// count is a hunk header's line count, which git omits when it is 1.
func count(s string) int {
	if s == "" {
		return 1
	}
	return atoi(s)
}
