//go:build unit

package config_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/hpcsc/vet/internal/config"
	"github.com/stretchr/testify/require"
)

func TestConfig(t *testing.T) {
	t.Run("parse", func(t *testing.T) {
		t.Run("an empty file gives an empty file", func(t *testing.T) {
			f, err := config.ParseFile("")

			require.NoError(t, err)
			require.Equal(t, config.File{}, f)
		})

		t.Run("takes the options from the file", func(t *testing.T) {
			f, err := config.ParseFile("apiKeyCommand: fnox get TYPESAFE_API_KEY\n" +
				"questionsFile: rules.yaml\n" +
				"apiUrl: https://example.com\n" +
				"model: rowan\n")

			require.NoError(t, err)
			c := f.Resolve()
			require.Equal(t, "fnox get TYPESAFE_API_KEY", c.APIKeyCommand)
			require.Equal(t, "rules.yaml", c.QuestionsFile)
			require.Equal(t, "https://example.com", c.APIURL)
			require.Equal(t, "rowan", c.Model)
		})

		t.Run("resolve fills the options a file never set", func(t *testing.T) {
			f, err := config.ParseFile("apiUrl: https://example.com")

			require.NoError(t, err)
			require.Equal(t, "jev-latest", f.Resolve().Model)
			require.Equal(t, "", f.Resolve().QuestionsFile)
		})

		t.Run("reads back the file that YAML writes", func(t *testing.T) {
			want := config.Default()
			want.APIKeyCommand = "fnox get TYPESAFE_API_KEY"
			want.QuestionsFile = "rules.yaml"

			text, err := want.YAML()

			require.NoError(t, err)
			got, err := config.ParseFile(string(text))
			require.NoError(t, err)
			require.Equal(t, want, got.Resolve())
		})
	})

	t.Run("merge", func(t *testing.T) {
		parse := func(t *testing.T, text string) config.File {
			t.Helper()
			f, err := config.ParseFile(text)
			require.NoError(t, err)
			return f
		}

		t.Run("the overlay keeps the options the base set where it sets none", func(t *testing.T) {
			base := parse(t, "model: base-model\nquestionsFile: base.yaml")
			over := parse(t, "model: over-model")

			merged := base.Merge(over).Resolve()

			require.Equal(t, "over-model", merged.Model)
			require.Equal(t, "base.yaml", merged.QuestionsFile)
		})

		t.Run("the overlay leaves an option untouched when neither sets it", func(t *testing.T) {
			merged := config.File{}.Merge(config.File{}).Resolve()

			require.Equal(t, config.DefaultAPIURL, merged.APIURL)
		})
	})

	t.Run("errors", func(t *testing.T) {
		t.Run("names each option that vet does not know", func(t *testing.T) {
			_, err := config.ParseFile("apiiKeyCommand: fnox\nmodel: rowan\nblah: x\n")

			require.EqualError(t, err, "unknown option apiiKeyCommand\nunknown option blah")
		})

		t.Run("a value that is not a string is an error", func(t *testing.T) {
			_, err := config.ParseFile("model: 5\n")

			require.Error(t, err)
		})

		t.Run("text that is not yaml is an error", func(t *testing.T) {
			_, err := config.ParseFile("model: [")

			require.Error(t, err)
		})
	})

	t.Run("load", func(t *testing.T) {
		write := func(t *testing.T, text string) string {
			t.Helper()
			path := filepath.Join(t.TempDir(), "config.yaml")
			require.NoError(t, os.WriteFile(path, []byte(text), 0o600))
			return path
		}

		t.Run("reads the options from the file", func(t *testing.T) {
			f, err := config.LoadFile(write(t, "model: rowan\n"))

			require.NoError(t, err)
			require.Equal(t, "rowan", f.Resolve().Model)
		})

		t.Run("a missing file gives an error that fs.ErrNotExist matches", func(t *testing.T) {
			_, err := config.LoadFile(filepath.Join(t.TempDir(), "config.yaml"))

			require.ErrorIs(t, err, fs.ErrNotExist)
		})

		t.Run("an error starts with the path of the file", func(t *testing.T) {
			path := write(t, "blah: x\n")

			_, err := config.LoadFile(path)

			require.EqualError(t, err, path+": unknown option blah")
		})

		t.Run("puts each of two or more errors on its own line under the path", func(t *testing.T) {
			path := write(t, "blah: x\nbloo: y\n")

			_, err := config.LoadFile(path)

			require.EqualError(t, err, path+":\n  unknown option blah\n  unknown option bloo")
		})
	})

	t.Run("path", func(t *testing.T) {
		env := func(vars map[string]string) func(string) string {
			return func(name string) string { return vars[name] }
		}

		t.Run("is in XDG_CONFIG_HOME when it is set", func(t *testing.T) {
			path := config.Path(env(map[string]string{"XDG_CONFIG_HOME": "/xdg", "HOME": "/home/ann"}))

			require.Equal(t, "/xdg/vet/config.yaml", path)
		})

		t.Run("is in ~/.config when XDG_CONFIG_HOME is not set", func(t *testing.T) {
			path := config.Path(env(map[string]string{"HOME": "/home/ann"}))

			require.Equal(t, "/home/ann/.config/vet/config.yaml", path)
		})

		t.Run("is in ~/.config when XDG_CONFIG_HOME is not an absolute path", func(t *testing.T) {
			path := config.Path(env(map[string]string{"XDG_CONFIG_HOME": "xdg", "HOME": "/home/ann"}))

			require.Equal(t, "/home/ann/.config/vet/config.yaml", path)
		})
	})

	t.Run("the default questions path", func(t *testing.T) {
		t.Run("sits next to the config file", func(t *testing.T) {
			path := config.QuestionsPath(func(name string) string {
				return map[string]string{"HOME": "/home/ann"}[name]
			})

			require.Equal(t, "/home/ann/.config/vet/questions.yaml", path)
		})
	})
}
