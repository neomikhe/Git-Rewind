// Package gitexec wraps the system "git" binary. It invokes plumbing commands
// (rev-parse, reflog, fsck, for-each-ref) with timeouts and parses their stable
// --format output, so git-rewind never reimplements Git internals.
package gitexec
