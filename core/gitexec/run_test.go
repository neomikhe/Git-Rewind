package gitexec

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestMutateIsNotBoundByTheReadTimeout(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH")
	}
	dir := t.TempDir()
	initRepo(t, dir)
	r := New(dir)
	r.timeout = time.Nanosecond
	ctx := context.Background()

	if _, err := r.Run(ctx, "status"); err == nil {
		t.Fatal("Run ignored its timeout")
	}
	if _, err := r.Mutate(ctx, "branch", "--list"); err != nil {
		t.Errorf("Mutate was cut short by the read timeout: %v", err)
	}
}

func TestMutateDoesNotStartOnACancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := New(t.TempDir()).Mutate(ctx, "status"); !errors.Is(err, context.Canceled) {
		t.Errorf("Mutate on a cancelled context = %v, want context.Canceled", err)
	}
}

func TestErrorsDoNotRevealTheRepositoryPath(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH")
	}
	dir := t.TempDir()
	initRepo(t, dir)
	missing := filepath.Join(dir, "missing.txt")

	_, err := New(dir).Run(context.Background(), "ls-files", "--error-unmatch", missing)
	if err == nil {
		t.Fatal("expected ls-files to fail on a missing path")
	}
	msg := err.Error()
	for _, leak := range []string{dir, filepath.ToSlash(dir)} {
		if strings.Contains(msg, leak) {
			t.Errorf("error reveals the local path %q: %s", leak, msg)
		}
	}
	if !strings.Contains(msg, "<repo>") {
		t.Errorf("error = %s, want the path shown as <repo>", msg)
	}
}

func TestErrorsShortenLongArgumentLists(t *testing.T) {
	args := []string{"log", "--no-walk"}
	for range 30 {
		args = append(args, strings.Repeat("a", 40))
	}
	got := describeArgs(args)
	if strings.Count(got, strings.Repeat("a", 40)) > maxArgsInError {
		t.Errorf("describeArgs kept too many arguments: %s", got)
	}
	if !strings.Contains(got, "(26 more)") {
		t.Errorf("describeArgs = %s, want it to say how many were left out", got)
	}
}

func TestErrorsHideTheRepositoryWhenRunFromASubdirectory(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH")
	}
	dir := t.TempDir()
	initRepo(t, dir)
	sub := filepath.Join(dir, "nested", "deeper")
	if err := os.MkdirAll(sub, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".git", "index.lock"), nil, 0o600); err != nil {
		t.Fatal(err)
	}

	runners := map[string]string{"subdirectory": sub}
	if runtime.GOOS == "windows" {
		runners["lower-cased path"] = strings.ToLower(sub)
	}
	for name, from := range runners {
		t.Run(name, func(t *testing.T) {
			_, err := New(from).Mutate(context.Background(), "add", ".")
			if err == nil {
				t.Fatal("expected git add to fail on a locked index")
			}
			msg := err.Error()
			for _, leak := range []string{dir, filepath.ToSlash(dir), strings.ToLower(filepath.ToSlash(dir))} {
				if strings.Contains(strings.ToLower(msg), strings.ToLower(leak)) {
					t.Errorf("error reveals the local path: %s", msg)
				}
			}
			if !strings.Contains(msg, "<repo>/.git/index.lock") {
				t.Errorf("error = %s, want the lock shown as <repo>/.git/index.lock", msg)
			}
		})
	}
}

func TestReplaceWholePathRespectsPathBoundaries(t *testing.T) {
	cases := []struct {
		text, path, label string
		fold              bool
		want              string
	}{
		{"open /home/ann/x and /home/anna/y", "/home/ann", "~", false, "open ~/x and /home/anna/y"},
		{"branch feature/app-login at /app/.git", "/app", "<repo>", false, "branch feature/app-login at <repo>/.git"},
		{"'/app'", "/app", "<repo>", false, "'<repo>'"},
		{"/app", "/app", "<repo>", false, "<repo>"},
		{"fatal: 'd:/Work/Ann/repo/.git/index.lock'", "D:/Work/Ann/repo", "<repo>", true, "fatal: '<repo>/.git/index.lock'"},
		{"fatal: 'd:/Work/Ann/repo/.git/index.lock'", "D:/Work/Ann/repo", "<repo>", false, "fatal: 'd:/Work/Ann/repo/.git/index.lock'"},
	}
	for _, c := range cases {
		if got := replaceWholePath(c.text, c.path, c.label, c.fold); got != c.want {
			t.Errorf("replaceWholePath(%q, %q, fold=%v) = %q, want %q", c.text, c.path, c.fold, got, c.want)
		}
	}
}

func TestGitNeverWaitsForATerminalPrompt(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH")
	}
	dir := t.TempDir()
	initRepo(t, dir)
	out, err := New(dir).Run(context.Background(), "-c", `alias.prompt=!echo "prompt=$GIT_TERMINAL_PROMPT"`, "prompt")
	if err != nil {
		t.Fatalf("running a shell alias: %v", err)
	}
	if got := strings.TrimSpace(out); got != "prompt=0" {
		t.Errorf("git's children see %q, want GIT_TERMINAL_PROMPT=0 so nothing waits for a prompt", got)
	}
}
