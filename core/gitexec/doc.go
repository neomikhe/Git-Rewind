// Package gitexec wraps the system "git" binary, running plumbing commands with timeouts
// and parsing their stable --format output so git-rewind never reimplements Git internals.
package gitexec
