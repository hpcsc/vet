package material

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/hpcsc/vet/internal/diff"
	"github.com/hpcsc/vet/internal/git"
	"github.com/hpcsc/vet/internal/questions"
)

// head is the revision the change is being judged against, so "the file before
// the change" means the base commit and not whatever the working directory
// happens to hold.
const head = "HEAD"

// Resolver turns the material a questions file named into sections for one
// changed file.
type Resolver struct {
	repo *git.Repo
}

func NewResolver(repo *git.Repo) *Resolver {
	return &Resolver{repo: repo}
}

// Resolve reads what the questions file asked for, in the order it defined, so
// the same rules produce the same prompt every run.
//
// A path a revision does not have contributes nothing rather than failing the
// run: a new file has no previous version, and a rule asking for one is better
// answered on what it has than not answered. A git failure is returned, because
// that means the material could not be read rather than that it is absent.
func (r *Resolver) Resolve(ctx context.Context, includes []questions.Include, f diff.File, base string) ([]Section, error) {
	dir := git.DirOf(f.Path)
	var sections []Section
	for _, include := range includes {
		content, err := r.resolveOne(ctx, include, f, dir, base)
		if err != nil {
			return nil, fmt.Errorf("resolve %s for %s: %w", include, f.Path, err)
		}
		if content == "" {
			continue
		}
		sections = append(sections, Section{Include: include, Content: content})
	}
	return cap(sections), nil
}

func (r *Resolver) resolveOne(ctx context.Context, include questions.Include, f diff.File, dir, base string) (string, error) {
	switch include {
	case questions.SiblingFilePaths:
		paths, err := r.pathsUnder(ctx, dir)
		if err != nil {
			return "", err
		}
		var others []string
		for _, path := range paths {
			if path != f.Path {
				others = append(others, path)
			}
		}
		return listing("Files in this directory:", others), nil
	case questions.ContainingDirDeclarations:
		return r.declarationsUnder(ctx, dir)
	case questions.ContainingDirContent:
		paths, err := r.pathsUnder(ctx, dir)
		if err != nil {
			return "", err
		}
		var kept []string
		var bodies []string
		for _, path := range paths {
			if path == f.Path {
				continue
			}
			content, err := r.read(ctx, head, path)
			if err != nil {
				return "", err
			}
			kept, bodies = append(kept, path), append(bodies, content)
		}
		return titled("Files in this directory:", kept, bodies), nil
	case questions.RepoDeclarations:
		return r.declarationsUnder(ctx, "")
	case questions.FileContent:
		return r.read(ctx, head, f.Path)
	case questions.PreviousFileContent:
		return r.read(ctx, base, f.Path)
	}
	return "", nil
}

// read returns a path's content at a revision, or nothing when the revision
// does not have it.
func (r *Resolver) read(ctx context.Context, rev, path string) (string, error) {
	content, err := r.repo.FileAt(ctx, rev, path)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	return content, err
}

func (r *Resolver) pathsUnder(ctx context.Context, dir string) ([]string, error) {
	return r.repo.FilesAt(ctx, head, dir)
}

func (r *Resolver) declarationsUnder(ctx context.Context, dir string) (string, error) {
	paths, err := r.pathsUnder(ctx, dir)
	if err != nil {
		return "", err
	}
	var kept []string
	var bodies []string
	for _, path := range paths {
		content, err := r.read(ctx, head, path)
		if err != nil {
			return "", err
		}
		if decls := declarations(path, content); decls != "" {
			kept, bodies = append(kept, path), append(bodies, decls)
		}
	}
	if len(kept) == 0 {
		return "", nil
	}
	return titled("Declarations in this directory:", kept, bodies), nil
}

func listing(heading string, lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	return heading + "\n" + strings.Join(lines, "\n")
}

func titled(heading string, paths, bodies []string) string {
	if len(paths) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(heading)
	b.WriteString("\n")
	for i, path := range paths {
		fmt.Fprintf(&b, "--- %s\n%s\n", path, bodies[i])
	}
	return strings.TrimRight(b.String(), "\n")
}
