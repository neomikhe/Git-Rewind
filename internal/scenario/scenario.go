// Package scenario builds reproducible "broken" Git repositories for git-rewind's
// integration tests, each with a hermetic environment and a deterministic clock.
package scenario

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const shortHashLen = 7

// Scenario describes one reproducible broken-repository fixture and how to verify it.
type Scenario struct {
	Name        string
	Description string
	Build       func(dir string) (Built, error)
	Verify      func(b Built) error
}

// Built is a built scenario: the repository path and its named hashes and refs.
type Built struct {
	Dir     string
	Anchors map[string]string
}

type repo struct {
	dir   string
	clock time.Time
	err   error
}

func newRepo(dir string) *repo {
	r := &repo{dir: dir, clock: time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)}
	r.git("init", "-q", "-b", "main")
	r.git("config", "commit.gpgsign", "false")
	r.git("config", "core.autocrlf", "false")
	r.git("config", "gc.auto", "0")
	return r
}

func (r *repo) git(args ...string) string {
	if r.err != nil {
		return ""
	}
	r.clock = r.clock.Add(time.Minute)
	stamp := r.clock.Format("2006-01-02T15:04:05 -0700")

	out, err := runGit(r.dir, []string{
		"GIT_AUTHOR_DATE=" + stamp,
		"GIT_COMMITTER_DATE=" + stamp,
		"GIT_AUTHOR_NAME=Rewind Test",
		"GIT_AUTHOR_EMAIL=test@example.invalid",
		"GIT_COMMITTER_NAME=Rewind Test",
		"GIT_COMMITTER_EMAIL=test@example.invalid",
	}, args...)
	if err != nil {
		r.err = err
	}
	return out
}

func (r *repo) writeFile(name, content string) {
	if r.err != nil {
		return
	}
	if err := os.WriteFile(filepath.Join(r.dir, name), []byte(content), 0o600); err != nil {
		r.err = err
	}
}

func (r *repo) commit(name, content, message string) string {
	r.writeFile(name, content)
	r.git("add", name)
	r.git("commit", "-q", "-m", message)
	return r.git("rev-parse", "HEAD")
}

func (r *repo) done(anchors map[string]string) (Built, error) {
	if r.err != nil {
		return Built{}, r.err
	}
	return Built{Dir: r.dir, Anchors: anchors}, nil
}

func runGit(dir string, extraEnv []string, args ...string) (string, error) {
	cmd := exec.Command("git", args...) //nolint:gosec // G204: fixed "git" command with package-controlled arguments.
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_CONFIG_GLOBAL="+os.DevNull,
		"GIT_CONFIG_SYSTEM="+os.DevNull,
		"GIT_TERMINAL_PROMPT=0",
	)
	cmd.Env = append(cmd.Env, extraEnv...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(stdout.String()), nil
}

func gitOut(dir string, args ...string) (string, error) {
	return runGit(dir, nil, args...)
}

func gitSucceeds(dir string, args ...string) bool {
	_, err := runGit(dir, nil, args...)
	return err == nil
}

func parents(dir, rev string) ([]string, error) {
	out, err := gitOut(dir, "rev-list", "--parents", "-n", "1", rev)
	if err != nil {
		return nil, err
	}
	fields := strings.Fields(out)
	if len(fields) == 0 {
		return nil, fmt.Errorf("no revision found for %q", rev)
	}
	return fields[1:], nil
}

func requireCommit(dir, hash string) error {
	typ, err := gitOut(dir, "cat-file", "-t", hash)
	if err != nil {
		return fmt.Errorf("object %s is missing: %w", short(hash), err)
	}
	if typ != "commit" {
		return fmt.Errorf("object %s is a %s, want a commit", short(hash), typ)
	}
	return nil
}

func short(hash string) string {
	if len(hash) > shortHashLen {
		return hash[:shortHashLen]
	}
	return hash
}
