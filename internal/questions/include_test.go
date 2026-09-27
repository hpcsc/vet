//go:build unit

package questions

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hpcsc/vet/internal/diff"
	"github.com/stretchr/testify/require"
)

const minimalRule = `
rules:
  - id: r
    type: noul
    noulLimit: 0.5
    instructions: do the thing
`

func parseWithInclude(t *testing.T, include string) (File, error) {
	t.Helper()
	return Parse([]byte("version: 1\ninclude: ["+include+"]\n"+minimalRule), t.TempDir())
}

func TestInclude(t *testing.T) {
	t.Run("validating", func(t *testing.T) {
		t.Run("accepts a name it knows", func(t *testing.T) {
			for _, name := range includes {
				file, err := parseWithInclude(t, name.String())
				require.NoError(t, err)
				require.Equal(t, []Include{name}, file.Include)
			}
		})

		t.Run("rejects a name it does not know, and lists the ones it does", func(t *testing.T) {
			_, err := parseWithInclude(t, "siblingPaths")
			require.Error(t, err)
			require.Contains(t, err.Error(), `"siblingPaths"`)
			require.Contains(t, err.Error(), "siblingFilePaths")
		})

		t.Run("rejects a name that reads like a grep", func(t *testing.T) {
			_, err := parseWithInclude(t, "grep:./internal/diff/*.go")
			require.Error(t, err)
		})

		t.Run("puts a single file's names in prompt order, not the order it wrote them", func(t *testing.T) {
			file, err := Parse([]byte("version: 1\ninclude: [previousFileContent, siblingFilePaths, fileContent]\n"+minimalRule), t.TempDir())
			require.NoError(t, err)
			require.Equal(t, []Include{SiblingFilePaths, FileContent, PreviousFileContent}, file.Include)
		})

		t.Run("accepts a questions file that asks for nothing", func(t *testing.T) {
			file, err := Parse([]byte("version: 1\n"+minimalRule), t.TempDir())
			require.NoError(t, err)
			require.Empty(t, file.Include)
		})
	})

	t.Run("union", func(t *testing.T) {
		t.Run("collects what several questions files asked for", func(t *testing.T) {
			union := unionIncludes([]Include{FileContent}, []Include{SiblingFilePaths})
			require.Equal(t, []Include{SiblingFilePaths, FileContent}, union)
		})

		t.Run("pays for the same material once", func(t *testing.T) {
			union := unionIncludes([]Include{FileContent}, []Include{FileContent})
			require.Equal(t, []Include{FileContent}, union)
		})

		t.Run("puts the material in prompt order, not the order it was asked for", func(t *testing.T) {
			union := unionIncludes(
				[]Include{PreviousFileContent, SiblingFilePaths},
				[]Include{ContainingDirContent},
			)
			require.Equal(t, []Include{SiblingFilePaths, ContainingDirContent, PreviousFileContent}, union)
		})

		t.Run("is empty when nothing was asked for", func(t *testing.T) {
			require.Empty(t, unionIncludes(nil, []Include{}))
		})
	})

	t.Run("scoping", func(t *testing.T) {
		t.Run("keeps the material a questions file asked for", func(t *testing.T) {
			dir := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(dir, "a.yaml"), []byte("version: 1\ninclude: [siblingFilePaths]\nfiles: [\"**/*.go\"]\n"+minimalRule), 0o644))
			file, err := Load(dir)
			require.NoError(t, err)
			require.Equal(t, []Include{SiblingFilePaths}, file.Include)
		})

		t.Run("gives a path the union of every questions file that matched it", func(t *testing.T) {
			dir := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(dir, "a.yaml"), []byte("version: 1\ninclude: [siblingFilePaths]\nfiles: [\"**/*.go\"]\n"+minimalRule), 0o644))
			require.NoError(t, os.WriteFile(filepath.Join(dir, "b.yaml"), []byte("version: 1\ninclude: [fileContent]\nfiles: [\"**/*.go\"]\nrules:\n  - id: s\n    type: noul\n    noulLimit: 0.5\n    instructions: do the other thing\n"), 0o644))
			file, err := Load(dir)
			require.NoError(t, err)
			require.Equal(t, []Include{SiblingFilePaths, FileContent}, file.Include)
		})
	})
}

func TestForPath(t *testing.T) {
	t.Run("gives", func(t *testing.T) {
		t.Run("the material the file asked for along with the rules that match", func(t *testing.T) {
			file, err := Parse([]byte("version: 1\ninclude: [siblingFilePaths, fileContent]\nfiles: [\"**/*.go\"]\n"+minimalRule), t.TempDir())
			require.NoError(t, err)
			scoped, ok := file.ForPath(diff.File{Path: "internal/a.go"})
			require.True(t, ok)
			require.Equal(t, []Include{SiblingFilePaths, FileContent}, scoped.Include)
			require.Len(t, scoped.Rules, 1)
		})
	})
}
