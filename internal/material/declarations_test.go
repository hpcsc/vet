//go:build unit

package material

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDeclarations(t *testing.T) {
	t.Run("reading", func(t *testing.T) {
		t.Run("names every top-level function, keeping whether its receiver is a pointer", func(t *testing.T) {
			src := `package a

func Exported() {}

func (r *Repo) Method(ctx context.Context) (string, error) {
	return "", nil
}
`
			require.Equal(t, "func Exported()\nfunc *Repo Method(ctx context.Context) (string, error)", declarations("a.go", src))
		})

		t.Run("keeps a type's definition, which is what a naming rule compares", func(t *testing.T) {
			src := `package a

type Blob struct{ Name string }
`
			require.Equal(t, "type Blob struct{ Name string }", declarations("a.go", src))
		})

		t.Run("names constants and variables without their values", func(t *testing.T) {
			src := `package a

const Name = "a very long value that is not worth sending"

var count = 42
`
			require.Equal(t, "const Name\nvar count", declarations("a.go", src))
		})

		t.Run("names each name in a grouped declaration", func(t *testing.T) {
			src := `package a

const (
	First  = 1
	Second = 2
)
`
			require.Equal(t, "const First\nconst Second", declarations("a.go", src))
		})

		t.Run("leaves out a function's body, which is most of the file", func(t *testing.T) {
			src := `package a

func Big() {
	lines := []string{
		"one",
		"two",
		"three",
	}
	_ = lines
}
`
			require.Equal(t, "func Big()", declarations("a.go", src))
		})
	})

	t.Run("declining", func(t *testing.T) {
		t.Run("gives nothing for a file that is not Go", func(t *testing.T) {
			require.Empty(t, declarations("README.md", "# a\n\nsome prose\n"))
		})

		t.Run("gives nothing for Go that does not parse", func(t *testing.T) {
			require.Empty(t, declarations("broken.go", "package a\n\nfunc (\n"))
		})

		t.Run("gives nothing for a file with no declarations", func(t *testing.T) {
			require.Empty(t, declarations("a.go", "package a\n"))
		})
	})
}
