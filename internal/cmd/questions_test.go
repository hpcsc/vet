//go:build unit

package cmd

import (
	"bytes"
	"context"
	"testing"

	"github.com/hpcsc/vet/internal/questions"
	"github.com/stretchr/testify/require"
)

func TestQuestionsCommand(t *testing.T) {
	t.Run("prints a valid example questions file with all three rule types", func(t *testing.T) {
		var out bytes.Buffer
		command := newQuestionsCommand()
		command.Writer = &out

		err := command.Run(context.Background(), []string{"questions"})

		require.NoError(t, err)
		file, err := questions.Parse(out.Bytes())
		require.NoError(t, err)
		require.Len(t, file.Rules, 3)
		require.Equal(t, questions.Noul, file.Rules[0].Type)
		require.Equal(t, questions.Choice, file.Rules[1].Type)
		require.Equal(t, questions.Score, file.Rules[2].Type)
	})
}
