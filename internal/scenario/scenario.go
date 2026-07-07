// Package scenario builds reproducible "broken" Git repositories used by
// git-rewind's integration tests. Each Scenario creates a specific wreckage
// (reset --hard, deleted branch, amend, aborted rebase, ...) with deterministic
// commits and timestamps, and can verify that the repository ended up in the
// expected broken state.
//
// The builders shell out to the system git binary with a hermetic environment
// (global and system config disabled, GC turned off, fixed identity and dates)
// so results are identical across machines and platforms, including Windows.
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

// Scenario describes one reproducible broken-repository fixture.
type Scenario struct {
	// Name is a stable, kebab-case identifier, e.g. "reset-hard".
	Name string
	// Description is a one-line summary of what went wrong.
	Description string
	// Build creates the scenario in dir (an existing, empty directory) and
	// returns the anchors a rescue test needs.
	Build func(dir string) (Built, error)
	// Verify confirms the repository is in the expected broken state. It is the
	// guard that keeps these fixtures trustworthy as the project evolves.
	Verify func(b Built) error
}

// Built is the result of building a scenario.
type Built struct {
	// Dir is the repository path.
	Dir string
	// Anchors holds scenario-specific named hashes and refs (for example
	// "lost" -> the hash of an orphaned commit). Each builder documents the
	// keys it sets.
	Anchors map[string]string
}

// repo is a small helper for building a scenario repository. It uses a sticky
// error: once any git command fails, later calls are no-ops and the first error
// is returned by done, keeping builder code free of repetitive error checks.
type repo struct {
	dir   string
	clock time.Time
	err   error
}

// newRepo initializes a fresh repository in dir with a deterministic clock and
// hermetic settings.
func newRepo(dir string) *repo {
	r := &repo{dir: dir, clock: time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)}
	r.git("init", "-q", "-b", "main")
	r.git("config", "commit.gpgsign", "false")
	r.git("config", "core.autocrlf", "false")
	r.git("config", "gc.auto", "0")
	return r
}

// git runs a git command in the repository, advancing the clock by a minute so
// each operation has a distinct, reproducible timestamp.
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

// writeFile writes content to a file inside the repository.
func (r *repo) writeFile(name, content string) {
	if r.err != nil {
		return
	}
	if err := os.WriteFile(filepath.Join(r.dir, name), []byte(content), 0o600); err != nil {
		r.err = err
	}
}

// commit writes a file, stages it, commits it, and returns the new HEAD hash.
func (r *repo) commit(name, content, message string) string {
	r.writeFile(name, content)
	r.git("add", name)
	r.git("commit", "-q", "-m", message)
	return r.git("rev-parse", "HEAD")
}

// done returns the built scenario, or the first git error that occurred.
func (r *repo) done(anchors map[string]string) (Built, error) {
	if r.err != nil {
		return Built{}, r.err
	}
	return Built{Dir: r.dir, Anchors: anchors}, nil
}

// runGit executes a git command in dir with a hermetic environment and returns
// its trimmed standard output.
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

// gitOut runs a read-only git query in dir and returns its trimmed output.
func gitOut(dir string, args ...string) (string, error) {
	return runGit(dir, nil, args...)
}

// gitSucceeds reports whether a git command exits zero.
func gitSucceeds(dir string, args ...string) bool {
	_, err := runGit(dir, nil, args...)
	return err == nil
}

// parents returns the parent hashes of a revision.
func parents(dir, rev string) ([]string, error) {
	out, err := gitOut(dir, "rev-list", "--parents", "-n", "1", rev)
	if err != nil {
		return nil, err
	}
	fields := strings.Fields(out)
	if len(fields) == 0 {
		return nil, fmt.Errorf("no revision found for %q", rev)
	}
	return fields[1:], nil // the first field is the commit itself
}

// requireCommit returns an error unless hash names an existing commit object,
// which is what makes a "lost" commit recoverable.
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

// short abbreviates a hash for readable error messages.
func short(hash string) string {
	if len(hash) > 7 {
		return hash[:7]
	}
	return hash
}
