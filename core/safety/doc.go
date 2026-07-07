// Package safety enforces git-rewind's safety guarantees: it creates a backup
// branch (backup/rewind-<timestamp>) before any destructive operation, verifies
// the working tree, and powers the dry-run mode that prints the exact commands
// before they run. Backups are created even when confirmation is skipped.
package safety
