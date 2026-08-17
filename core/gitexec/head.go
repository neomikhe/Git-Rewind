package gitexec

import (
	"context"
	"strings"
)

// Head is where a repository's HEAD currently points.
type Head struct {
	Hash     string
	Branch   string
	Detached bool
}

// HeadState reports the commit HEAD points at and the branch it is on, if any.
func (r *Runner) HeadState(ctx context.Context) (Head, error) {
	hash, err := r.Run(ctx, "rev-parse", "HEAD")
	if err != nil {
		return Head{}, err
	}
	head := Head{Hash: strings.TrimSpace(hash)}

	branch, err := r.run(ctx, "symbolic-ref", "-q", "--short", "HEAD")
	if err != nil {
		if isNoMatch(err) {
			head.Detached = true
			return head, nil
		}
		return Head{}, err
	}
	head.Branch = strings.TrimSpace(string(branch))
	return head, nil
}
