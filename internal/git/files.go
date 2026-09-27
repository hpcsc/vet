package git

import (
	"context"
	"fmt"
	"os"
	"path"
	"strings"
)

// FilesAt lists the paths under dir at a revision, as paths from the root of the
// repository. An empty dir means the whole repository.
//
// This reads the tree at the revision rather than the working directory, so
// what a rule is told about the repository is the commit the change is being
// judged against, not whatever happens to be on disk while the judge runs.
func (r *Repo) FilesAt(ctx context.Context, rev, dir string) ([]string, error) {
	args := []string{"ls-tree", "-r", "--name-only", rev}
	if dir != "" {
		args = append(args, "--", dir)
	}
	out, err := r.run(ctx, args...)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, line := range strings.Split(out, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}

// FileAt returns the content of a path at a revision. A path the revision does
// not have is `os.ErrNotExist`, which is what a rule comparing a new file
// against what was there needs to hear: the file did not exist before. A git
// failure is a different thing and is returned as itself.
func (r *Repo) FileAt(ctx context.Context, rev, filePath string) (string, error) {
	has, err := r.HasFileAt(ctx, rev, filePath)
	if err != nil {
		return "", err
	}
	if !has {
		return "", fmt.Errorf("%s at %s: %w", filePath, rev, os.ErrNotExist)
	}
	out, err := r.run(ctx, "show", rev+":"+filePath)
	if err != nil {
		return "", err
	}
	return out, nil
}

// HasFileAt reports whether a revision has a path.
//
// The revision is checked on its own first, because a revision that does not
// resolve and a path that is not there both make the combined lookup exit 1.
// Reporting the second as the first would turn a mistake in the base ref into a
// quiet "this file has no previous version", which is a wrong answer to the
// question a rule is asking rather than a missing one.
func (r *Repo) HasFileAt(ctx context.Context, rev, filePath string) (bool, error) {
	ok, err := r.ok(ctx, "rev-parse", "--verify", "--quiet", rev)
	if err != nil {
		return false, err
	}
	if !ok {
		return false, fmt.Errorf("the revision %s does not resolve", rev)
	}
	return r.ok(ctx, "rev-parse", "--verify", "--quiet", rev+":"+filePath)
}

// DirOf is the directory a path sits in, as a path from the root of the
// repository. A path at the root has no directory, so the root is its own
// answer.
func DirOf(filePath string) string {
	dir := path.Dir(filePath)
	if dir == "." {
		return ""
	}
	return dir
}
