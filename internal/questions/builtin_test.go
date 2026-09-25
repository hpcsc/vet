//go:build unit

package questions

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefault(t *testing.T) {
	t.Run("gives a valid file with practical rules of every kind", func(t *testing.T) {
		file, err := Default()

		require.NoError(t, err)
		require.Equal(t, 1, file.Version)
		require.Len(t, file.Rules, 6)
		require.Equal(t, "tests-through-public-api", file.Rules[0].ID)
		require.Equal(t, "rejected-operation-inert", file.Rules[1].ID)
		require.Equal(t, "comment-adds-guidance", file.Rules[2].ID)
		require.Equal(t, "test-double", file.Rules[3].ID)
		require.Equal(t, "public-contract-change", file.Rules[4].ID)
		require.Equal(t, "testing-quality", file.Rules[5].ID)
		require.ElementsMatch(t, []Kind{Noul, Noul, Noul, Choice, Choice, Score}, []Kind{
			file.Rules[0].Type,
			file.Rules[1].Type,
			file.Rules[2].Type,
			file.Rules[3].Type,
			file.Rules[4].Type,
			file.Rules[5].Type,
		})
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
