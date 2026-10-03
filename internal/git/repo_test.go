//go:build unit

package git

import (
	"context"
	"path/filepath"
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

		t.Run("falls back to HEAD~1 when no branch resolves", func(t *testing.T) {
			repo := gittest.NewLocal(t)
			repo.Git("checkout", "-q", "--detach")
			repo.Git("update-ref", "-d", "refs/heads/main")

			base, err := New(repo.Dir).DetectBase(ctx, "")

			require.NoError(t, err)
			require.Equal(t, "HEAD~1", base)
		})

		t.Run("fails when no base resolves", func(t *testing.T) {
			repo := gittest.Empty(t)

			_, err := New(repo.Dir).DetectBase(ctx, "")

			require.Error(t, err)
		})
	})

	t.Run("toplevel", func(t *testing.T) {
		t.Run("returns the root of the working tree", func(t *testing.T) {
			repo := gittest.NewLocal(t)
			repo.Write("deep/file.txt", "x\n")

			top, err := New(repo.Dir).Toplevel(ctx)

			require.NoError(t, err)
			// git reports the resolved path, and on macOS the temp dir is a
			// symlink, so /var and /private/var name the same folder. Compare
			// the resolved paths so the two agree.
			resolved, err := filepath.EvalSymlinks(repo.Dir)
			require.NoError(t, err)
			require.Equal(t, resolved, top)
		})

		t.Run("fails outside a repository", func(t *testing.T) {
			_, err := New(t.TempDir()).Toplevel(ctx)

			require.Error(t, err)
		})
	})

	t.Run("changes", func(t *testing.T) {
		t.Run("lists every file the head changed", func(t *testing.T) {
			repo := gittest.NewWithRemote(t)
			repo.Commit("one.go", "one\n", "Add one.go")
			repo.Commit("two.go", "two\n", "Add two.go")

			changes, err := New(repo.Dir).Changes(ctx, "origin/main")

			require.NoError(t, err)
			require.Equal(t, []Change{{Path: "one.go"}, {Path: "two.go"}}, changes)
		})

		t.Run("gives the destination path of a rename", func(t *testing.T) {
			repo := gittest.NewWithRemote(t)
			repo.Git("mv", "code.go", "moved.go")
			repo.Git("commit", "-q", "-m", "Rename code.go")

			changes, err := New(repo.Dir).Changes(ctx, "origin/main")

			require.NoError(t, err)
			require.Equal(t, []Change{{Path: "moved.go", From: "code.go"}}, changes)
		})

		t.Run("lists a deleted file", func(t *testing.T) {
			repo := gittest.NewWithRemote(t)
			repo.Git("rm", "code.go")
			repo.Git("commit", "-q", "-m", "Delete code.go")

			changes, err := New(repo.Dir).Changes(ctx, "origin/main")

			require.NoError(t, err)
			require.Equal(t, []Change{{Path: "code.go"}}, changes)
		})

		t.Run("gives no previous path for a file that stays put", func(t *testing.T) {
			repo := gittest.NewWithRemote(t)
			repo.Commit("code.go", "one\n", "Add code.go")
			repo.Commit("code.go", "two\n", "Change code.go")

			changes, err := New(repo.Dir).Changes(ctx, "origin/main")

			require.NoError(t, err)
			require.Equal(t, []Change{{Path: "code.go"}}, changes)
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

		t.Run("pairs a rename when both paths are named", func(t *testing.T) {
			repo := gittest.NewWithRemote(t)
			repo.Git("mv", "code.go", "moved.go")
			repo.Git("commit", "-q", "-m", "Rename code.go")

			diff, err := New(repo.Dir).UnifiedDiff(ctx, "origin/main", "moved.go", "code.go")

			require.NoError(t, err)
			require.Contains(t, diff, "rename from code.go")
			require.Contains(t, diff, "rename to moved.go")
		})

		t.Run("hides the rename headers when only the destination is named", func(t *testing.T) {
			repo := gittest.NewWithRemote(t)
			repo.Git("mv", "code.go", "moved.go")
			repo.Git("commit", "-q", "-m", "Rename code.go")

			diff, err := New(repo.Dir).UnifiedDiff(ctx, "origin/main", "moved.go")

			require.NoError(t, err)
			require.NotContains(t, diff, "rename from")
		})
	})
}
