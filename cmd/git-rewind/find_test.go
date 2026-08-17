package main

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestFindReportsTheLostCommitAndHowToKeepIt(t *testing.T) {
	dir, lost := resetHardRepo(t)

	var buf bytes.Buffer
	if err := run([]string{"find", "second commit"}, dir, &buf); err != nil {
		t.Fatalf("run find: %v", err)
	}

	out := buf.String()
	for _, want := range []string{
		"Matches for",
		"1 commit",
		"second commit",
		"the commit message matches",
		"git branch " + rescuePrefix + shortHash(lost) + " " + lost,
		"only adds a branch",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("find output is missing %q\n---\n%s", want, out)
		}
	}
}

func TestFindMatchesFileContents(t *testing.T) {
	dir, _ := resetHardRepo(t)

	var buf bytes.Buffer
	if err := run([]string{"find", "two"}, dir, &buf); err != nil {
		t.Fatalf("run find: %v", err)
	}
	if !strings.Contains(buf.String(), "file.txt:") {
		t.Errorf("find should report the matching file and line\n---\n%s", buf.String())
	}
}

func TestFindWithoutAQueryIsAnError(t *testing.T) {
	var buf bytes.Buffer
	if err := run([]string{"find"}, ".", &buf); err == nil {
		t.Fatal("expected an error when no text is given")
	}
}

func TestFindReportsNoMatch(t *testing.T) {
	dir, _ := resetHardRepo(t)

	var buf bytes.Buffer
	if err := run([]string{"find", "nothing-like-this-exists"}, dir, &buf); err != nil {
		t.Fatalf("run find: %v", err)
	}
	if !strings.Contains(buf.String(), "nothing matching") {
		t.Errorf("unexpected output: %q", buf.String())
	}
}

func TestFindOnARepoWithNothingLost(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH")
	}
	dir := t.TempDir()
	git := hermeticGit(t, dir)
	git("init", "-q", "-b", "main")
	git("config", "commit.gpgsign", "false")
	writeFile(t, dir, "f.txt", "only\n")
	git("add", "f.txt")
	git("commit", "-q", "-m", "only commit")

	var buf bytes.Buffer
	if err := run([]string{"find", "anything"}, dir, &buf); err != nil {
		t.Fatalf("run find: %v", err)
	}
	if !strings.Contains(buf.String(), "no unreachable commits") {
		t.Errorf("unexpected output: %q", buf.String())
	}
}

func TestClipKeepsOutputReadable(t *testing.T) {
	long := strings.Repeat("x", maxMatchLineLen+50)
	got := clip(long)
	if utf8.RuneCountInString(got) != maxMatchLineLen+3 || !strings.HasSuffix(got, "...") {
		t.Errorf("clip returned %d runes, want %d plus an ellipsis", utf8.RuneCountInString(got), maxMatchLineLen)
	}
	if short := clip("short"); short != "short" {
		t.Errorf("clip shortened a line that fits: %q", short)
	}
}

// TestClipNeverSplitsARune guards against slicing a matched line by byte. find greps whatever
// is in the user's files, so a line of CJK, accented or emoji text would otherwise be cut
// mid-character and printed as invalid UTF-8.
func TestClipNeverSplitsARune(t *testing.T) {
	for _, char := range []string{"á", "€", "漢", "🙂", "x"} {
		line := strings.Repeat(char, maxMatchLineLen+40)
		got := clip(line)

		if !utf8.ValidString(got) {
			t.Errorf("clip(%q repeated) produced invalid UTF-8", char)
		}
		if want := maxMatchLineLen + 3; utf8.RuneCountInString(got) != want {
			t.Errorf("clip(%q repeated) kept %d runes, want %d — the limit counts characters, not bytes",
				char, utf8.RuneCountInString(got), want)
		}
	}
}
