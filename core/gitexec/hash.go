package gitexec

import "fmt"

const (
	sha1HexLen   = 40
	sha256HexLen = 64
)

func isFullHash(s string) bool {
	if len(s) != sha1HexLen && len(s) != sha256HexLen {
		return false
	}
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

func requireFullHashes(hashes []string) error {
	for _, h := range hashes {
		if !isFullHash(h) {
			return fmt.Errorf("not a full commit hash: %q", h)
		}
	}
	return nil
}
