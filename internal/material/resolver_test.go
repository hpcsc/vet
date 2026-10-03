//go:build unit

package material

import (
	"context"
	"strings"
	"testing"

	"github.com/hpcsc/vet/internal/diff"
	"github.com/hpcsc/vet/internal/git"
	"github.com/hpcsc/vet/internal/gittest"
	"github.com/hpcsc/vet/internal/questions"
	"github.com/stretchr/testify/require"
)

// content is the shorthand for a subtest that only cares what came back: one
// file, the includes it named, and the one section that answered.
func content(t *testing.T, repo *gittest.Repo, includes []questions.Include, path, base string) string {
	t.Helper()
	sections, err := NewResolver(git.New(repo.Dir)).Resolve(context.Background(), includes, diff.File{Path: path}, base)
	require.NoError(t, err)
	require.Len(t, sections, 1)
	return sections[0].Content
}

func TestResolver(t *testing.T) {
	ctx := context.Background()
	repoWithFiles := func(t *testing.T) *gittest.Repo {
		repo := gittest.NewLocal(t)
		repo.Commit("internal/diff/file.go", "package diff\n\ntype File struct{ Path string }\n", "Add file.go")
		repo.Commit("internal/diff/loader_test.go", "package diff\n\nfunc TestLoad(t *testing.T) {}\n", "Add loader_test.go")
		repo.Commit("internal/cmd/judge.go", "package cmd\n\nfunc Judge() {}\n", "Add judge.go")
		return repo
	}
	resolve := func(t *testing.T, repo *gittest.Repo, includes []questions.Include, path, base string) []Section {
		t.Helper()
		sections, err := NewResolver(git.New(repo.Dir)).Resolve(ctx, includes, diff.File{Path: path}, base)
		require.NoError(t, err)
		return sections
	}

	t.Run("sibling file paths", func(t *testing.T) {
		t.Run("names the other files in the same directory", func(t *testing.T) {
			repo := repoWithFiles(t)
			content := content(t, repo, []questions.Include{questions.SiblingFilePaths}, "internal/diff/file.go", "HEAD~1")
			require.Equal(t, "Files in this directory:\ninternal/diff/loader_test.go", content)
		})

		t.Run("leaves out the file being judged, which the prompt already carries", func(t *testing.T) {
			repo := repoWithFiles(t)
			content := content(t, repo, []questions.Include{questions.SiblingFilePaths}, "internal/diff/file.go", "HEAD~1")
			require.NotContains(t, content, "file.go")
		})

		t.Run("gives nothing for a directory holding only the judged file", func(t *testing.T) {
			repo := repoWithFiles(t)
			require.Empty(t, resolve(t, repo, []questions.Include{questions.SiblingFilePaths}, "internal/cmd/judge.go", "HEAD~1"))
		})
	})

	t.Run("containing dir declarations", func(t *testing.T) {
		t.Run("gives the declarations in the same directory, without their bodies", func(t *testing.T) {
			repo := repoWithFiles(t)
			content := content(t, repo, []questions.Include{questions.ContainingDirDeclarations}, "internal/diff/file.go", "HEAD~1")
			require.Contains(t, content, "type File struct{ Path string }")
			require.Contains(t, content, "func TestLoad(t *testing.T)")
			require.NotContains(t, content, "package diff")
		})

		t.Run("leaves out the bodies", func(t *testing.T) {
			repo := gittest.NewLocal(t)
			repo.Commit("a/a.go", "package a\n\nfunc Big() {\n\tx := 1\n\t_ = x\n}\n", "Add a.go")
			content := content(t, repo, []questions.Include{questions.ContainingDirDeclarations}, "a/a.go", "HEAD~1")
			require.Equal(t, "Declarations in this directory:\n--- a/a.go\nfunc Big()", content)
		})

		t.Run("says which file each declaration came from", func(t *testing.T) {
			repo := repoWithFiles(t)
			content := content(t, repo, []questions.Include{questions.ContainingDirDeclarations}, "internal/diff/file.go", "HEAD~1")
			require.Contains(t, content, "--- internal/diff/file.go")
			require.Contains(t, content, "--- internal/diff/loader_test.go")
		})
	})

	t.Run("containing dir content", func(t *testing.T) {
		t.Run("gives the whole text of the other files in the directory", func(t *testing.T) {
			repo := repoWithFiles(t)
			content := content(t, repo, []questions.Include{questions.ContainingDirContent}, "internal/diff/file.go", "HEAD~1")
			require.Contains(t, content, "--- internal/diff/loader_test.go")
			require.Contains(t, content, "func TestLoad(t *testing.T) {}")
		})
	})

	t.Run("repo declarations", func(t *testing.T) {
		t.Run("gives the declarations from every directory", func(t *testing.T) {
			repo := repoWithFiles(t)
			content := content(t, repo, []questions.Include{questions.RepoDeclarations}, "internal/diff/file.go", "HEAD~1")
			require.Contains(t, content, "type File struct{ Path string }")
			require.Contains(t, content, "func Judge()")
		})
	})

	t.Run("file content", func(t *testing.T) {
		t.Run("gives the whole file, not just the changed lines", func(t *testing.T) {
			repo := repoWithFiles(t)
			content := content(t, repo, []questions.Include{questions.FileContent}, "internal/diff/file.go", "HEAD~1")
			require.Contains(t, content, "type File struct{ Path string }")
		})
	})

	t.Run("previous file content", func(t *testing.T) {
		t.Run("gives the file as it was at the base", func(t *testing.T) {
			repo := gittest.NewLocal(t)
			repo.Commit("a.go", "package a\n", "Add a.go")
			base := trim(repo.Git("rev-parse", "HEAD"))
			repo.Commit("a.go", "package a\n\nconst Added = 1\n", "Add a const")

			content := content(t, repo, []questions.Include{questions.PreviousFileContent}, "a.go", base)

			require.Equal(t, "package a\n", content)
		})

		t.Run("gives nothing for a file the base did not have, so a new file is not an error", func(t *testing.T) {
			repo := gittest.NewLocal(t)
			repo.Commit("old.go", "package old\n", "Add old.go")
			base := trim(repo.Git("rev-parse", "HEAD"))
			repo.Commit("new.go", "package new\n", "Add new.go")

			sections := resolve(t, repo, []questions.Include{questions.PreviousFileContent}, "new.go", base)

			require.Empty(t, sections)
		})
	})

	t.Run("ordering", func(t *testing.T) {
		t.Run("keeps the order it was given, which the questions file fixed", func(t *testing.T) {
			repo := repoWithFiles(t)
			sections := resolve(t, repo,
				[]questions.Include{questions.FileContent, questions.SiblingFilePaths},
				"internal/diff/file.go", "HEAD~1")
			require.Len(t, sections, 2)
			require.Equal(t, questions.FileContent, sections[0].Include)
			require.Equal(t, questions.SiblingFilePaths, sections[1].Include)
		})
	})

	t.Run("asking for", func(t *testing.T) {
		t.Run("nothing gives nothing, so a questions file that wants none pays none", func(t *testing.T) {
			repo := repoWithFiles(t)
			require.Empty(t, resolve(t, repo, nil, "internal/diff/file.go", "HEAD~1"))
		})

		t.Run("a revision that does not resolve fails, rather than reading as an empty file", func(t *testing.T) {
			repo := repoWithFiles(t)
			_, err := NewResolver(git.New(repo.Dir)).Resolve(ctx,
				[]questions.Include{questions.PreviousFileContent}, diff.File{Path: "internal/diff/file.go"}, "no-such-rev")
			require.Error(t, err)
			require.Contains(t, err.Error(), "does not resolve")
		})
	})
}

func TestSection(t *testing.T) {
	t.Run("bytes", func(t *testing.T) {
		t.Run("is what the material adds to the prompt", func(t *testing.T) {
			sections := []Section{{Include: questions.FileContent, Content: "abcde"}, {Content: "xy"}}
			require.Equal(t, 7, Bytes(sections))
		})

		t.Run("is nothing when there is no material", func(t *testing.T) {
			require.Zero(t, Bytes(nil))
		})
	})

	t.Run("cap", func(t *testing.T) {
		t.Run("leaves material under the limit alone", func(t *testing.T) {
			sections := []Section{{Include: questions.FileContent, Content: "short"}}
			require.Equal(t, sections, cap(sections))
		})

		t.Run("cuts the section that crosses the limit and drops the rest", func(t *testing.T) {
			big := strings.Repeat("x", maxMaterialBytes)
			sections := []Section{
				{Include: questions.SiblingFilePaths, Content: "names"},
				{Include: questions.FileContent, Content: big},
				{Include: questions.PreviousFileContent, Content: "dropped"},
			}

			kept := cap(sections)

			require.Len(t, kept, 2)
			require.Equal(t, questions.SiblingFilePaths, kept[0].Include)
			require.Equal(t, questions.FileContent, kept[1].Include)
			require.True(t, strings.HasSuffix(kept[1].Content, "..."))
			require.LessOrEqual(t, Bytes(kept), maxMaterialBytes)
		})
	})

	t.Run("render", func(t *testing.T) {
		t.Run("labels each section with the file and the piece it came from", func(t *testing.T) {
			rendered := Render([]Section{{Include: questions.SiblingFilePaths, Content: "Files in this directory:\na.go"}}, "internal/diff/file.go")
			require.Equal(t, "Repository material for internal/diff/file.go (siblingFilePaths):\n\nFiles in this directory:\na.go", rendered)
		})

		t.Run("keeps the sections in order", func(t *testing.T) {
			rendered := Render([]Section{
				{Include: questions.SiblingFilePaths, Content: "first"},
				{Include: questions.FileContent, Content: "second"},
			}, "a.go")
			require.Contains(t, rendered, "first")
			require.Less(t, indexOf(rendered, "first"), indexOf(rendered, "second"))
		})

		t.Run("leaves out a section with nothing in it", func(t *testing.T) {
			rendered := Render([]Section{{Include: questions.FileContent, Content: ""}}, "a.go")
			require.Empty(t, rendered)
		})

		t.Run("is nothing when there is no material", func(t *testing.T) {
			require.Empty(t, Render(nil, "a.go"))
		})
	})
}

func trim(s string) string { return strings.TrimSpace(s) }

func indexOf(haystack, needle string) int { return strings.Index(haystack, needle) }
