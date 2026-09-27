package diff

import (
	"context"

	"github.com/hpcsc/vet/internal/git"
)

type Loader struct {
	repo *git.Repo
}

func NewLoader(repo *git.Repo) *Loader {
	return &Loader{repo: repo}
}

func (l *Loader) Load(ctx context.Context, base string) ([]File, error) {
	changes, err := l.repo.Changes(ctx, base)
	if err != nil {
		return nil, err
	}
	files := make([]File, 0, len(changes))
	for _, change := range changes {
		// git only pairs a rename when it sees both paths, so a moved file gets
		// the other path too and the patch keeps its rename headers.
		paths := []string{change.Path}
		if change.From != "" {
			paths = append(paths, change.From)
		}
		text, err := l.repo.UnifiedDiff(ctx, base, paths...)
		if err != nil {
			return nil, err
		}
		files = append(files, File{Path: change.Path, From: change.From, Diff: text})
	}
	return files, nil
}
