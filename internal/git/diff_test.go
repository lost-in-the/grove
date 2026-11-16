package git

import (
	"fmt"
	"strings"
	"testing"
)

// sig is a compact form of a diff line: kind, old and new numbers, text.
func sig(l DiffLine) string {
	kind := map[LineKind]string{LineContext: " ", LineAdded: "+", LineRemoved: "-", LineNoNewline: `\`}[l.Kind]
	return fmt.Sprintf("%s%d,%d %s", kind, l.OldNo, l.NewNo, l.Text)
}

func sigs(h Hunk) []string {
	var out []string
	for _, l := range h.Lines {
		out = append(out, sig(l))
	}
	return out
}

func parseOne(t *testing.T, diff string) FileDiff {
	t.Helper()
	files, err := ParseDiff(diff)
	if err != nil {
		t.Fatalf("ParseDiff: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("got %d files, want 1", len(files))
	}
	return files[0]
}

func wantSigs(t *testing.T, h Hunk, want ...string) {
	t.Helper()
	if got := sigs(h); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("lines:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestParseDiff_ModifiedTwoHunks(t *testing.T) {
	f := parseOne(t, "diff --git a/main.go b/main.go\n"+
		"index 1111111..2222222 100644\n"+
		"--- a/main.go\n"+
		"+++ b/main.go\n"+
		"@@ -1,3 +1,4 @@ package main\n"+
		" package main\n"+
		"+import \"fmt\"\n"+
		" \n"+
		" func a() {}\n"+
		"@@ -10,2 +11,2 @@ func b() {\n"+
		"-\told()\n"+
		"+\tnew()\n"+
		" }\n")
	if f.Status != FileModified || f.Path() != "main.go" || f.Added != 2 || f.Removed != 1 {
		t.Errorf("file = %+v", f)
	}
	if len(f.Hunks) != 2 {
		t.Fatalf("got %d hunks, want 2", len(f.Hunks))
	}
	h := f.Hunks[0]
	if h.OldStart != 1 || h.OldLines != 3 || h.NewStart != 1 || h.NewLines != 4 || h.Header != "@@ -1,3 +1,4 @@ package main" {
		t.Errorf("hunk 0 = %+v", h)
	}
	wantSigs(t, h, " 1,1 package main", "+0,2 import \"fmt\"", " 2,3 ", " 3,4 func a() {}")
	wantSigs(t, f.Hunks[1], "-10,0 \told()", "+0,11 \tnew()", " 11,12 }")
}

func TestParseDiff_AddedFile(t *testing.T) {
	f := parseOne(t, "diff --git a/new.txt b/new.txt\n"+
		"new file mode 100644\n"+
		"index 0000000..3333333\n"+
		"--- /dev/null\n"+
		"+++ b/new.txt\n"+
		"@@ -0,0 +1,2 @@\n"+
		"+one\n"+
		"+two\n")
	if f.Status != FileAdded || f.OldPath != "" || f.Path() != "new.txt" || f.Added != 2 {
		t.Errorf("file = %+v", f)
	}
	wantSigs(t, f.Hunks[0], "+0,1 one", "+0,2 two")
}

func TestParseDiff_DeletedFile(t *testing.T) {
	f := parseOne(t, "diff --git a/old.txt b/old.txt\n"+
		"deleted file mode 100644\n"+
		"index 3333333..0000000\n"+
		"--- a/old.txt\n"+
		"+++ /dev/null\n"+
		"@@ -1 +0,0 @@\n"+
		"-gone\n")
	if f.Status != FileDeleted || f.NewPath != "" || f.Path() != "old.txt" || f.Removed != 1 {
		t.Errorf("file = %+v", f)
	}
	wantSigs(t, f.Hunks[0], "-1,0 gone")
}

func TestParseDiff_RenameWithEdit(t *testing.T) {
	f := parseOne(t, "diff --git a/a.txt b/b.txt\n"+
		"similarity index 80%\n"+
		"rename from a.txt\n"+
		"rename to b.txt\n"+
		"index 4444444..5555555 100644\n"+
		"--- a/a.txt\n"+
		"+++ b/b.txt\n"+
		"@@ -1 +1 @@\n"+
		"-x\n"+
		"+y\n")
	if f.Status != FileRenamed || f.OldPath != "a.txt" || f.NewPath != "b.txt" {
		t.Errorf("file = %+v", f)
	}
	wantSigs(t, f.Hunks[0], "-1,0 x", "+0,1 y")
}

func TestParseDiff_PureRename(t *testing.T) {
	f := parseOne(t, "diff --git a/c.txt b/d.txt\n"+
		"similarity index 100%\n"+
		"rename from c.txt\n"+
		"rename to d.txt\n")
	if f.Status != FileRenamed || f.OldPath != "c.txt" || f.NewPath != "d.txt" || len(f.Hunks) != 0 {
		t.Errorf("file = %+v", f)
	}
}

func TestParseDiff_Binary(t *testing.T) {
	f := parseOne(t, "diff --git a/x.png b/x.png\n"+
		"index 6666666..7777777 100644\n"+
		"Binary files a/x.png and b/x.png differ\n")
	if !f.Binary || f.Path() != "x.png" || len(f.Hunks) != 0 {
		t.Errorf("file = %+v", f)
	}
}

func TestParseDiff_NoNewlineMarker(t *testing.T) {
	f := parseOne(t, "diff --git a/n.txt b/n.txt\n"+
		"--- a/n.txt\n"+
		"+++ b/n.txt\n"+
		"@@ -1 +1 @@\n"+
		"-a\n"+
		"\\ No newline at end of file\n"+
		"+b\n"+
		"\\ No newline at end of file\n")
	wantSigs(t, f.Hunks[0], "-1,0 a", `\0,0 No newline at end of file`, "+0,1 b", `\0,0 No newline at end of file`)
}

// A hunk ends by its counts: content that looks like a file header is a
// removed or added line, and the next file still parses.
func TestParseDiff_DashPrefixedContent(t *testing.T) {
	files, err := ParseDiff("diff --git a/d.md b/d.md\n" +
		"--- a/d.md\n" +
		"+++ b/d.md\n" +
		"@@ -1,2 +1,2 @@\n" +
		"--- x\n" +
		"+++ y\n" +
		" keep\n" +
		"diff --git a/e.md b/e.md\n" +
		"--- a/e.md\n" +
		"+++ b/e.md\n" +
		"@@ -1 +1 @@\n" +
		"-p\n" +
		"+q\n")
	if err != nil || len(files) != 2 {
		t.Fatalf("files %d, err %v", len(files), err)
	}
	wantSigs(t, files[0].Hunks[0], "-1,0 -- x", "+0,1 ++ y", " 2,2 keep")
	if files[1].Path() != "e.md" {
		t.Errorf("second file = %+v", files[1])
	}
}

func TestParseDiff_QuotedPath(t *testing.T) {
	f := parseOne(t, "diff --git \"a/sp ace\\t.go\" \"b/sp ace\\t.go\"\n"+
		"--- \"a/sp ace\\t.go\"\n"+
		"+++ \"b/sp ace\\t.go\"\n"+
		"@@ -1 +1 @@\n"+
		"-a\n"+
		"+b\n")
	if f.Path() != "sp ace\t.go" {
		t.Errorf("path = %q", f.Path())
	}
}

// git ends a ---/+++ path holding a space with a tab, so tools can tell
// where the name stops; the tab is not part of the name.
func TestParseDiff_PathWithSpaceTrailingTab(t *testing.T) {
	f := parseOne(t, "diff --git a/sp ace.txt b/sp ace.txt\n"+
		"--- a/sp ace.txt\t\n"+
		"+++ b/sp ace.txt\t\n"+
		"@@ -1 +1 @@\n"+
		"-a\n"+
		"+b\n")
	if f.OldPath != "sp ace.txt" || f.NewPath != "sp ace.txt" {
		t.Errorf("paths %q, %q", f.OldPath, f.NewPath)
	}
}

func TestParseDiff_RawUTF8Path(t *testing.T) {
	f := parseOne(t, "diff --git a/café.go b/café.go\n"+
		"--- a/café.go\n"+
		"+++ b/café.go\n"+
		"@@ -1 +1 @@\n"+
		"-a\n"+
		"+b\n")
	if f.Path() != "café.go" {
		t.Errorf("path = %q", f.Path())
	}
}

func TestParseDiff_Empty(t *testing.T) {
	files, err := ParseDiff("")
	if files != nil || err != nil {
		t.Errorf("got %v, %v; want nil, nil", files, err)
	}
}

func TestParseDiff_Malformed(t *testing.T) {
	for name, diff := range map[string]string{
		"bad hunk header": "diff --git a/m b/m\n--- a/m\n+++ b/m\n@@ bogus @@\n",
		"truncated hunk":  "diff --git a/m b/m\n--- a/m\n+++ b/m\n@@ -1,3 +1,3 @@\n a\n",
	} {
		_, err := ParseDiff(diff)
		if err == nil || !strings.Contains(err.Error(), "line 4") {
			t.Errorf("%s: err = %v, want one naming line 4", name, err)
		}
	}
	// A line that is not a diff line is named by its own number.
	_, err := ParseDiff("diff --git a/m b/m\n--- a/m\n+++ b/m\n@@ -1,2 +1,2 @@\n a\n*b\n")
	if err == nil || !strings.Contains(err.Error(), "line 6") {
		t.Errorf("bad line in hunk: err = %v, want one naming line 6", err)
	}
}
