package gitexec

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	defaultTimeout    = 10 * time.Second
	maxArgsInError    = 6
	minPrivatePathLen = 3
	repoLabel         = "<repo>"
	homeLabel         = "~"
	pathOpeners       = " \t\n'\"`(="
	pathClosers       = "/ \t\n'\"`):" + string(filepath.Separator)
	noTerminalPrompt  = "GIT_TERMINAL_PROMPT=0"
)

// Runner executes git plumbing commands inside one repository.
type Runner struct {
	dir         string
	timeout     time.Duration
	privateOnce sync.Once
	private     []privatePath
}

type privatePath struct {
	path  string
	label string
}

// New returns a Runner that runs git in dir, a repository root or any path inside it.
func New(dir string) *Runner {
	return &Runner{dir: dir, timeout: defaultTimeout}
}

// Run executes a git command that only reads, bounded by a short timeout, and returns its standard output.
func (r *Runner) Run(ctx context.Context, args ...string) (string, error) {
	out, err := r.run(ctx, args...)
	return string(out), err
}

// Mutate executes a git command that changes the repository, with no timeout and ignoring cancellation once started.
func (r *Runner) Mutate(ctx context.Context, args ...string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	out, err := r.invoke(context.WithoutCancel(ctx), args...)
	return string(out), err
}

func (r *Runner) run(ctx context.Context, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	return r.invoke(ctx, args...)
}

func (r *Runner) invoke(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", args...) //nolint:gosec // G204: fixed "git" command, non-shell args controlled by this package.
	cmd.Dir = r.dir
	cmd.Env = append(os.Environ(), noTerminalPrompt)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git %s: %w: %s", r.redact(describeArgs(args)), err, r.redact(printableLines(stderr.String())))
	}
	return stdout.Bytes(), nil
}

func describeArgs(args []string) string {
	if len(args) <= maxArgsInError {
		return Printable(strings.Join(args, " "))
	}
	shown := Printable(strings.Join(args[:maxArgsInError], " "))
	return fmt.Sprintf("%s … (%d more)", shown, len(args)-maxArgsInError)
}

func printableLines(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	for i, line := range lines {
		lines[i] = Printable(strings.TrimRight(line, "\r"))
	}
	return strings.Join(lines, "\n")
}

func (r *Runner) redact(s string) string {
	r.privateOnce.Do(func() { r.private = r.resolvePrivatePaths() })
	fold := runtime.GOOS == "windows"
	for _, p := range r.private {
		for _, variant := range []string{p.path, filepath.ToSlash(p.path)} {
			s = replaceWholePath(s, variant, p.label, fold)
		}
	}
	return s
}

func (r *Runner) resolvePrivatePaths() []privatePath {
	var paths []privatePath
	add := func(path, label string) {
		if len(path) >= minPrivatePathLen {
			paths = append(paths, privatePath{path: filepath.Clean(path), label: label})
		}
	}
	if abs, err := filepath.Abs(r.dir); err == nil {
		add(abs, repoLabel)
	}
	for _, root := range r.repositoryRoots() {
		add(root, repoLabel)
	}
	if home, err := os.UserHomeDir(); err == nil {
		add(home, homeLabel)
	}
	sort.SliceStable(paths, func(i, j int) bool { return len(paths[i].path) > len(paths[j].path) })
	return paths
}

func (r *Runner) repositoryRoots() []string {
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--show-toplevel", "--git-common-dir") //nolint:gosec // G204: fixed "git" command and fixed arguments.
	cmd.Dir = r.dir
	cmd.Env = append(os.Environ(), noTerminalPrompt)
	out, err := cmd.Output()
	if err != nil {
		return nil
	}

	var roots []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		root := filepath.FromSlash(strings.TrimSpace(line))
		if root == "" {
			continue
		}
		if !filepath.IsAbs(root) {
			root = filepath.Join(r.dir, root)
		}
		if abs, err := filepath.Abs(root); err == nil {
			root = abs
		}
		if filepath.Base(root) == ".git" {
			root = filepath.Dir(root)
		}
		roots = append(roots, root)
	}
	return roots
}

func replaceWholePath(s, path, label string, fold bool) string {
	var b strings.Builder
	from := 0
	for {
		i := indexPath(s, path, from, fold)
		if i < 0 {
			b.WriteString(s[from:])
			return b.String()
		}
		end := i + len(path)
		if opensPath(s, i) && closesPath(s, end) {
			b.WriteString(s[from:i])
			b.WriteString(label)
		} else {
			b.WriteString(s[from:end])
		}
		from = end
	}
}

func indexPath(s, path string, from int, fold bool) int {
	haystack, needle := s[from:], path
	if fold {
		haystack, needle = asciiLower(haystack), asciiLower(needle)
	}
	i := strings.Index(haystack, needle)
	if i < 0 {
		return -1
	}
	return from + i
}

func asciiLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + ('a' - 'A')
		}
	}
	return string(b)
}

func opensPath(s string, i int) bool {
	return i == 0 || strings.IndexByte(pathOpeners, s[i-1]) >= 0
}

func closesPath(s string, end int) bool {
	return end == len(s) || strings.IndexByte(pathClosers, s[end]) >= 0
}
