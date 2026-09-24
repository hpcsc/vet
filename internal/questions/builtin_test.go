//go:build unit

package questions

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefault(t *testing.T) {
	t.Run("gives a valid file with every rule kind", func(t *testing.T) {
		file, err := Default()

		require.NoError(t, err)
		require.Equal(t, 1, file.Version)
		require.Len(t, file.Rules, 3)
		require.ElementsMatch(t, []Kind{Noul, Choice, Score}, []Kind{file.Rules[0].Type, file.Rules[1].Type, file.Rules[2].Type})
	})

	t.Run("round-trips through the file a directory would load", func(t *testing.T) {
		file, err := Default()
		require.NoError(t, err)

		text, err := Marshal(file)

		require.NoError(t, err)
		again, err := Parse(text, "")
		require.NoError(t, err)
		require.Equal(t, file, again)
	})
}