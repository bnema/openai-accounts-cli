package cmd

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommandRequestedJSONLastValueWins(t *testing.T) {
	root := &cobra.Command{Use: "oa"}
	root.PersistentFlags().Bool("json", false, "json output")

	assert.False(t, commandRequestedJSON(root, []string{"--json", "--json=false"}))
	assert.True(t, commandRequestedJSON(root, []string{"--json=false", "--json=true"}))
}

func TestExecuteRootCommandUsesProvidedArgs(t *testing.T) {
	root := &cobra.Command{Use: "oa"}
	root.PersistentFlags().Bool("json", false, "json output")

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	root.SetOut(stdout)
	root.SetErr(stderr)

	ranChild := false
	root.AddCommand(&cobra.Command{
		Use: "child",
		RunE: func(cmd *cobra.Command, args []string) error {
			ranChild = true
			return errors.New("boom")
		},
	})

	err := executeRootCommand(root, []string{"child", "--json"})
	require.Error(t, err)
	assert.True(t, ranChild)
	assert.JSONEq(t, `{"ok":false,"error":"boom"}`, strings.TrimSpace(stderr.String()))
}
