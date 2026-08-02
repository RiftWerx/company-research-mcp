package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInstallSkillBoth(t *testing.T) {
	tmp := t.TempDir()
	var buf bytes.Buffer
	cmd := InstallSkillCmd{Dest: tmp, out: &buf}
	require.NoError(t, cmd.Run())

	for _, name := range []string{"company-research-mcp", "company-research-cli"} {
		info, err := os.Stat(filepath.Join(tmp, name, "SKILL.md"))
		require.NoError(t, err)
		assert.Positive(t, info.Size())
		assert.Contains(t, buf.String(), filepath.Join(name, "SKILL.md"))
	}
}

func TestInstallSkillMCPOnly(t *testing.T) {
	tmp := t.TempDir()
	var buf bytes.Buffer
	cmd := InstallSkillCmd{Dest: tmp, MCP: true, out: &buf}
	require.NoError(t, cmd.Run())

	_, err := os.Stat(filepath.Join(tmp, "company-research-mcp", "SKILL.md"))
	require.NoError(t, err)
	assert.Contains(t, buf.String(), filepath.Join("company-research-mcp", "SKILL.md"))

	_, err = os.Stat(filepath.Join(tmp, "company-research-cli", "SKILL.md"))
	assert.True(t, os.IsNotExist(err))
	assert.NotContains(t, buf.String(), "company-research-cli")
}

func TestInstallSkillCLIOnly(t *testing.T) {
	tmp := t.TempDir()
	var buf bytes.Buffer
	cmd := InstallSkillCmd{Dest: tmp, CLI: true, out: &buf}
	require.NoError(t, cmd.Run())

	_, err := os.Stat(filepath.Join(tmp, "company-research-cli", "SKILL.md"))
	require.NoError(t, err)
	assert.Contains(t, buf.String(), filepath.Join("company-research-cli", "SKILL.md"))

	_, err = os.Stat(filepath.Join(tmp, "company-research-mcp", "SKILL.md"))
	assert.True(t, os.IsNotExist(err))
	assert.NotContains(t, buf.String(), "company-research-mcp")
}

func TestInstallSkillBothFlags(t *testing.T) {
	tmp := t.TempDir()
	var buf bytes.Buffer
	cmd := InstallSkillCmd{Dest: tmp, MCP: true, CLI: true, out: &buf}
	require.NoError(t, cmd.Run())

	for _, name := range []string{"company-research-mcp", "company-research-cli"} {
		info, err := os.Stat(filepath.Join(tmp, name, "SKILL.md"))
		require.NoError(t, err)
		assert.Positive(t, info.Size())
		assert.Contains(t, buf.String(), filepath.Join(name, "SKILL.md"))
	}
}

func TestInstallSkillDestMissing(t *testing.T) {
	cmd := InstallSkillCmd{Dest: "/nonexistent/path/that/cannot/exist"}
	err := cmd.Run()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "install skill")
}

func TestInstallSkillIdempotent(t *testing.T) {
	tmp := t.TempDir()
	var buf bytes.Buffer
	cmd := InstallSkillCmd{Dest: tmp, out: &buf}
	require.NoError(t, cmd.Run())

	out1 := buf.String()
	buf.Reset()
	require.NoError(t, cmd.Run())

	assert.Equal(t, out1, buf.String())
}

func TestInstallSkillDestIsFile(t *testing.T) {
	f, err := os.CreateTemp("", "notadir")
	require.NoError(t, err)
	_ = f.Close()
	t.Cleanup(func() { os.Remove(f.Name()) })

	cmd := InstallSkillCmd{Dest: f.Name()}
	err = cmd.Run()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a directory")
}
