//go:build unit

package jev

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCappedDiff(t *testing.T) {
	t.Run("leaves a diff under the cap alone", func(t *testing.T) {
		require.Equal(t, "small", cappedDiff("small"))
	})

	t.Run("truncates a diff over the cap and marks the cut", func(t *testing.T) {
		big := strings.Repeat("x", maxDiffBytes+100)

		got := cappedDiff(big)

		require.LessOrEqual(t, len(got), maxDiffBytes)
		require.Equal(t, strings.Repeat("x", maxDiffBytes-3)+"...", got)
	})
}
