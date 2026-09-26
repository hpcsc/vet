//go:build unit

package cmd

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBaseOf(t *testing.T) {
	t.Run("takes the ref from the flag when the flag is given", func(t *testing.T) {
		require.Equal(t, "origin/main", baseOf("origin/main", "release/1.2"))
	})

	t.Run("takes the ref from the bare argument when the flag is empty", func(t *testing.T) {
		require.Equal(t, "release/1.2", baseOf("", "release/1.2"))
	})

	t.Run("leaves the base to detection when neither is given", func(t *testing.T) {
		require.Empty(t, baseOf("", ""))
	})
}
