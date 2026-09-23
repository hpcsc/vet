//go:build unit

package git

import (
	"context"
	"testing"

	"github.com/hpcsc/vet/internal/gittest"
	"github.com/stretchr/testify/require"
)

func TestRepo(t *testing.T) {
	ctx := context.Background()

	t.Run("detect base", func(t *testing.T) {
		t.Run("uses the explicit base when it resolves", func(t *testing.T) {
			repo := gittest.NewLocal(t)

			base, err := New(repo.Dir).DetectBase(ctx, "main")

			require.NoError(t, err)
			require.Equal(t, "main", base)
		})

		t.Run("falls through to a default when the explicit base does not resolve", func(t *testing.T) {
			repo := gittest.NewLocal(t)

			base, err := New(repo.Dir).DetectBase(ctx, "no-such-ref")

			require.NoError(t, err)
			require.Equal(t, "main", base)
		})

		t.Run("falls back to origin/HEAD when no base is explicit", func(t *testing.T) {
			repo := gittest.NewWithRemote(t)

			base, err := New(repo.Dir).DetectBase(ctx, "")

			require.NoError(t, err)
			require.Equal(t, "origin/HEAD", base)
		})

		t.Run("falls back to origin/main when origin/HEAD is missing", func(t *testing.T) {
			repo := gittest.NewWithRemote(t)
			repo.Git("symbolic-ref", "-d", "refs/remotes/origin/HEAD")

			base, err := New(repo.Dir).DetectBase(ctx, "")

			require.NoError(t, err)
			require.Equal(t, "origin/main", base)
		})

		t.Run("falls back to origin/master when the other origin refs are missing", func(t *testing.T) {
			repo := gittest.NewWithRemote(t)
			repo.Git("push", "-q", "origin", "main:master")
			repo.Git("symbolic-ref", "-d", "refs/remotes/origin/HEAD")
			repo.Git("update-ref", "-d", "refs/remotes/origin/main")

			base, err := New(repo.Dir).DetectBase(ctx, "")

			require.NoError(t, err)
			require.Equal(t, "origin/master", base)
		})

		t.Run("falls back to main in a repository without a remote", func(t *testing.T) {
			repo := gittest.NewLocal(t)

			base, err := New(repo.Dir).DetectBase(ctx, "")

			require.NoError(t, err)
			require.Equal(t, "main", base)
		})

		t.Run("falls back to master when only master resolves", func(t *testing.T) {
			repo := gittest.NewLocal(t)
			repo.Git("branch", "master", "main")
			repo.Git("symbolic-ref", "HEAD", "refs/heads/master")
			repo.Git("update-ref", "-d", "refs/heads/main")

			base, err := New(repo.Dir).DetectBase(ctx, "")

			require.NoError(t, err)
			require.Equal(t, "master", base)
		})

		t.Run("fails when no base resolves", func(t *testing.T) {
			repo := gittest.Empty(t)

			_, err := New(repo.Dir).DetectBase(ctx, "")

			require.Error(t, err)
		})
	})

	t.Run("changed files", func(t *testing.T) {
		t.Run("lists every file the head changed", func(t *testing.T) {
			repo := gittest.NewWithRemote(t)
			repo.Commit("one.go", "one\n", "Add one.go")
			repo.Commit("two.go", "two\n", "Add two.go")

			paths, err := New(repo.Dir).ChangedFiles(ctx, "origin/main")

			require.NoError(t, err)
			require.Equal(t, []string{"one.go", "two.go"}, paths)
		})

		t.Run("gives the destination path of a rename", func(t *testing.T) {
			repo := gittest.NewWithRemote(t)
			repo.Git("mv", "code.go", "moved.go")
			repo.Git("commit", "-q", "-m", "Rename code.go")

			paths, err := New(repo.Dir).ChangedFiles(ctx, "origin/main")

			require.NoError(t, err)
			require.Equal(t, []string{"moved.go"}, paths)
		})

		t.Run("lists a deleted file", func(t *testing.T) {
			repo := gittest.NewWithRemote(t)
			repo.Git("rm", "code.go")
			repo.Git("commit", "-q", "-m", "Delete code.go")

			paths, err := New(repo.Dir).ChangedFiles(ctx, "origin/main")

			require.NoError(t, err)
			require.Equal(t, []string{"code.go"}, paths)
		})
	})

	t.Run("unified diff", func(t *testing.T) {
		t.Run("scopes the diff to the requested file", func(t *testing.T) {
			repo := gittest.NewWithRemote(t)
			repo.Commit("file-a.txt", "base a\n", "Add file-a.txt")
			repo.Commit("file-b.txt", "base b\n", "Add file-b.txt")
			repo.Commit("file-a.txt", "changed a\n", "Change file-a.txt")

			diff, err := New(repo.Dir).UnifiedDiff(ctx, "origin/main", "file-a.txt")

			require.NoError(t, err)
			require.Contains(t, diff, "b/file-a.txt")
			require.NotContains(t, diff, "file-b.txt")
		})

		t.Run("gives the diff of a deleted file", func(t *testing.T) {
			repo := gittest.NewWithRemote(t)
			repo.Git("rm", "code.go")
			repo.Git("commit", "-q", "-m", "Delete code.go")

			diff, err := New(repo.Dir).UnifiedDiff(ctx, "origin/main", "code.go")

			require.NoError(t, err)
			require.Contains(t, diff, "code.go")
		})
	})
}
