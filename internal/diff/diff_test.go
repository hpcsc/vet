//go:build unit

package diff_test

import (
	"context"
	"testing"

	"github.com/hpcsc/vet/internal/diff"
	"github.com/hpcsc/vet/internal/git"
	"github.com/hpcsc/vet/internal/gittest"
	"github.com/stretchr/testify/require"
)

func newLoader(t *testing.T, repo *gittest.Repo) *diff.Loader {
	t.Helper()
	return diff.NewLoader(git.New(repo.Dir))
}

func TestLoader(t *testing.T) {
	ctx := context.Background()

	t.Run("load", func(t *testing.T) {
		t.Run("builds one diff per changed file", func(t *testing.T) {
			repo := gittest.NewWithRemote(t)
			repo.Commit("a.txt", "one\n", "Add a.txt")
			repo.Commit("b.txt", "two\n", "Add b.txt")

			files, err := newLoader(t, repo).Load(ctx, "origin/main")

			require.NoError(t, err)
			require.Len(t, files, 2)
			require.Equal(t, "a.txt", files[0].Path)
			require.Contains(t, files[0].Diff, "a.txt")
			require.Equal(t, "b.txt", files[1].Path)
			require.Contains(t, files[1].Diff, "b.txt")
		})

		t.Run("uses the destination path in the diff of a rename", func(t *testing.T) {
			repo := gittest.NewWithRemote(t)
			repo.Git("mv", "code.go", "moved.go")
			repo.Git("commit", "-q", "-m", "Rename code.go")

			files, err := newLoader(t, repo).Load(ctx, "origin/main")

			require.NoError(t, err)
			require.Len(t, files, 1)
			require.Equal(t, "moved.go", files[0].Path)
			require.Contains(t, files[0].Diff, "moved.go")
		})

		t.Run("builds no diff when no file changed", func(t *testing.T) {
			repo := gittest.NewWithRemote(t)

			files, err := newLoader(t, repo).Load(ctx, "origin/main")

			require.NoError(t, err)
			require.Empty(t, files)
		})
	})
}
