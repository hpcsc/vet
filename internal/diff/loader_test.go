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

		t.Run("keeps the rename headers of a moved file", func(t *testing.T) {
			repo := gittest.NewWithRemote(t)
			repo.Git("mv", "code.go", "moved.go")
			repo.Git("commit", "-q", "-m", "Rename code.go")

			files, err := newLoader(t, repo).Load(ctx, "origin/main")

			require.NoError(t, err)
			require.Len(t, files, 1)
			require.Equal(t, "moved.go", files[0].Path)
			require.Equal(t, "code.go", files[0].From)
			require.Contains(t, files[0].Diff, "rename from code.go")
			require.Contains(t, files[0].Diff, "rename to moved.go")
		})

		t.Run("leaves no previous path on a file that stays put", func(t *testing.T) {
			repo := gittest.NewWithRemote(t)
			repo.Commit("code.go", "one\n", "Add code.go")
			repo.Commit("code.go", "two\n", "Change code.go")

			files, err := newLoader(t, repo).Load(ctx, "origin/main")

			require.NoError(t, err)
			require.Len(t, files, 1)
			require.Equal(t, "code.go", files[0].Path)
			require.Empty(t, files[0].From)
		})

		t.Run("builds no diff when no file changed", func(t *testing.T) {
			repo := gittest.NewWithRemote(t)

			files, err := newLoader(t, repo).Load(ctx, "origin/main")

			require.NoError(t, err)
			require.Empty(t, files)
		})
	})
}

func TestFile(t *testing.T) {
	t.Run("added lines", func(t *testing.T) {
		t.Run("gives the lines the change adds without the plus", func(t *testing.T) {
			file := diff.File{Diff: "@@ -1,1 +1,2 @@\n package x\n+// added\n+var y = 1\n"}

			require.Equal(t, []string{"// added", "var y = 1"}, file.AddedLines())
		})

		t.Run("leaves out the file header that also starts with a plus", func(t *testing.T) {
			file := diff.File{Diff: "diff --git a/x b/x\n--- a/x\n+++ b/x\n@@ -0,0 +1 @@\n+package x\n"}

			require.Equal(t, []string{"package x"}, file.AddedLines())
		})

		t.Run("keeps a line that only looks like a header once the hunk starts", func(t *testing.T) {
			file := diff.File{Diff: "+++ b/x\n@@ -0,0 +1 @@\n++++ not a header\n"}

			require.Equal(t, []string{"+++ not a header"}, file.AddedLines())
		})

		t.Run("leaves out the lines the change removes", func(t *testing.T) {
			file := diff.File{Diff: "@@ -1,2 +1 @@\n-// gone\n+// kept\n"}

			require.Equal(t, []string{"// kept"}, file.AddedLines())
		})

		t.Run("gives nothing for a diff with no hunk", func(t *testing.T) {
			file := diff.File{Diff: "diff --git a/x b/x\nsimilarity index 100%\nrename from x\nrename to y\n"}

			require.Empty(t, file.AddedLines())
		})

		t.Run("gives nothing for an empty diff", func(t *testing.T) {
			require.Empty(t, diff.File{}.AddedLines())
		})
	})
}
