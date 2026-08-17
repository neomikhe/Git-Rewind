// Package safety enforces git-rewind's guarantees: a backup branch before any destructive
// operation, dry-run by default, and working-tree checks. Backups are never skipped.
package safety
