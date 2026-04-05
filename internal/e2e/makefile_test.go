package e2e

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMakefileIncludesBuildAndInstallTargets(t *testing.T) {
	contents, err := os.ReadFile(filepath.Join(repoRoot(t), "Makefile"))
	require.NoError(t, err)

	makefile := string(contents)
	assert.Contains(t, makefile, "build:")
	assert.Contains(t, makefile, "install:")
	assert.Contains(t, makefile, ".PHONY: build install")
}
