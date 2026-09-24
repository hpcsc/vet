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
	paths, err := l.repo.ChangedFiles(ctx, base)
	if err != nil {
		return nil, err
	}
	files := make([]File, 0, len(paths))
	for _, path := range paths {
		text, err := l.repo.UnifiedDiff(ctx, base, path)
		if err != nil {
			return nil, err
		}
		files = append(files, File{Path: path, Diff: text})
	}
	return files, nil
}