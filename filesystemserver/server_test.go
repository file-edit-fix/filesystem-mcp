package filesystemserver_test

import (
	"testing"

	"github.com/bigmanBass666/filesystem-mcp/filesystemserver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// regression test: modify_file must be present with correct schema
func TestModifyFileSchema(t *testing.T) {
	fsserver, err := filesystemserver.NewFilesystemServer([]string{t.TempDir()})
	require.NoError(t, err)

	mcpClient := startTestClient(t, fsserver)

	tool := getTool(t, mcpClient, "modify_file")
	require.NotNil(t, tool)

	// make sure that the tool has the required schema fields
	_, ok := tool.InputSchema.Properties["path"]
	assert.True(t, ok)
	_, ok = tool.InputSchema.Properties["find"]
	assert.True(t, ok)
	_, ok = tool.InputSchema.Properties["replace"]
	assert.True(t, ok)
	_, ok = tool.InputSchema.Properties["dry_run"]
	assert.True(t, ok)
}
