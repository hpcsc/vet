//go:build unit

package git

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/hpcsc/vet/internal/gittest"
	"github.com/stretchr/testify/require"
)

func TestFiles(t *testing.T) {
	ctx := context.Background()

	t.Run("listing", func(t *testing.T) {
		t.Run("names the files under a directory", func(t *testing.T) {
			repo := gittest.NewLocal(t)
			repo.Commit("internal/diff/file.go", "package diff\n", "Add file.go")
			repo.Commit("internal/diff/loader.go", "package diff\n", "Add loader.go")
			repo.Commit("internal/cmd/judge.go", "package cmd\n", "Add judge.go")

			files, err := New(repo.Dir).FilesAt(ctx, "HEAD", "internal/diff")

			require.NoError(t, err)
			require.Equal(t, []string{"internal/diff/file.go", "internal/diff/loader.go"}, files)
		})

		t.Run("names every file when given no directory", func(t *testing.T) {
			repo := gittest.NewLocal(t)
			repo.Commit("internal/diff/file.go", "package diff\n", "Add file.go")
			repo.Commit("top.go", "package top\n", "Add top.go")

			files, err := New(repo.Dir).FilesAt(ctx, "HEAD", "")

			require.NoError(t, err)
			require.Equal(t, []string{"README.md", "code.go", "internal/diff/file.go", "top.go"}, files)
		})

		t.Run("reads the tree at the revision, not the working directory", func(t *testing.T) {
			repo := gittest.NewLocal(t)
			repo.Commit("kept.go", "package kept\n", "Add kept.go")
			base := head(t, repo)
			repo.Commit("later.go", "package later\n", "Add later.go")

			atBase, err := New(repo.Dir).FilesAt(ctx, base, "")
			require.NoError(t, err)
			require.NotContains(t, atBase, "later.go")

			atHead, err := New(repo.Dir).FilesAt(ctx, "HEAD", "")
			require.NoError(t, err)
			require.Contains(t, atHead, "later.go")
		})

		t.Run("is empty for a directory the revision does not have", func(t *testing.T) {
			repo := gittest.NewLocal(t)

			files, err := New(repo.Dir).FilesAt(ctx, "HEAD", "nowhere")

			require.NoError(t, err)
			require.Empty(t, files)
		})

		t.Run("fails on a revision that does not resolve", func(t *testing.T) {
			repo := gittest.NewLocal(t)

			_, err := New(repo.Dir).FilesAt(ctx, "no-such-rev", "")

			require.Error(t, err)
		})

		t.Run("reports a revision that does not resolve as an error, not as an empty tree", func(t *testing.T) {
			repo := gittest.NewLocal(t)

			_, err := New(repo.Dir).FileAt(ctx, "no-such-rev", "a.go")

			require.Error(t, err)
			require.Contains(t, err.Error(), "does not resolve")
		})
	})

	t.Run("reading", func(t *testing.T) {
		t.Run("returns a file's content at a revision", func(t *testing.T) {
			repo := gittest.NewLocal(t)
			repo.Commit("a.go", "package a\n\nconst Name = \"one\"\n", "Add a.go")

			content, err := New(repo.Dir).FileAt(ctx, "HEAD", "a.go")

			require.NoError(t, err)
			require.Equal(t, "package a\n\nconst Name = \"one\"\n", content)
		})

		t.Run("returns the content as it was before the change", func(t *testing.T) {
			repo := gittest.NewLocal(t)
			repo.Commit("a.go", "package a\n", "Add a.go")
			base := head(t, repo)
			repo.Commit("a.go", "package a\n\nconst Added = 1\n", "Add a const")

			content, err := New(repo.Dir).FileAt(ctx, base, "a.go")

			require.NoError(t, err)
			require.Equal(t, "package a\n", content)
		})

		t.Run("reports a path the revision does not have as not existing", func(t *testing.T) {
			repo := gittest.NewLocal(t)

			_, err := New(repo.Dir).FileAt(ctx, "HEAD", "never-written.go")

			require.Error(t, err)
			require.True(t, errors.Is(err, os.ErrNotExist))
		})
	})

	t.Run("dir of", func(t *testing.T) {
		t.Run("is the directory a path sits in", func(t *testing.T) {
			require.Equal(t, "internal/diff", DirOf("internal/diff/file.go"))
		})

		t.Run("is nothing for a path at the root", func(t *testing.T) {
			require.Empty(t, DirOf("main.go"))
		})
	})
}

func head(t testing.TB, repo *gittest.Repo) string {
	t.Helper()
	return strings.TrimSpace(repo.Git("rev-parse", "HEAD"))
}
