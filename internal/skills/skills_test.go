package skills

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFilesContainsBothSkills(t *testing.T) {
	_, err := Files.ReadFile(NameMCP + "/SKILL.md")
	require.NoError(t, err)

	_, err = Files.ReadFile(NameCLI + "/SKILL.md")
	require.NoError(t, err)
}
