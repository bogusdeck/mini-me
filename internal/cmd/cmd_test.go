package cmd_test

import (
	"bytes"
	"testing"

	"memex/internal/cmd"
)

func TestVersionCmd(t *testing.T) {
	buf := new(bytes.Buffer)
	cmd.RootCmd.SetOut(buf)
	cmd.RootCmd.SetArgs([]string{"version"})

	err := cmd.RootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
