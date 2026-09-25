//go:build unit

package style

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStyle(t *testing.T) {
	t.Run("leaves the text alone when the terminal cannot show color", func(t *testing.T) {
		setColor(t, false)

		require.Equal(t, "change.txt", File("change.txt"))
		require.Equal(t, FailMark, Fail(FailMark))
		require.Equal(t, "vet", Bold("vet"))
		require.Equal(t, "q quit", Faint("q quit"))
	})

	t.Run("gives each part of a report its own color", func(t *testing.T) {
		setColor(t, true)

		require.Equal(t, "\x1b[33mchange.txt\x1b[m", File("change.txt"))
		require.Equal(t, "\x1b[36mrules\x1b[m", Group("rules"))
		require.Equal(t, "\x1b[35mnoul\x1b[m", Type("noul"))
		require.Equal(t, "\x1b[34mno-secrets\x1b[m", Rule("no-secrets"))
		require.Equal(t, "\x1b[32m"+PassMark+"\x1b[m", Pass(PassMark))
		require.Equal(t, "\x1b[31m"+FailMark+"\x1b[m", Fail(FailMark))
	})

	t.Run("sets the weight the interactive view needs", func(t *testing.T) {
		setColor(t, true)

		require.Equal(t, "\x1b[1mvet\x1b[m", Bold("vet"))
		require.Equal(t, "\x1b[2mq quit\x1b[m", Faint("q quit"))
	})
}

func setColor(t *testing.T, on bool) {
	t.Helper()
	previous := useColor
	useColor = on
	t.Cleanup(func() { useColor = previous })
}
