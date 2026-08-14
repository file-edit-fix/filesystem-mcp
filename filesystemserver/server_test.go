package filesystemserver_test

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/bigmanBass666/filesystem-mcp/filesystemserver"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModifyFileSchema(t *testing.T) {
	fsserver, err := filesystemserver.NewFilesystemServer([]string{t.TempDir()})
	require.NoError(t, err)

	mcpClient := startTestClient(t, fsserver)

	tool := getTool(t, mcpClient, "modify_file")
	require.NotNil(t, tool)

	_, ok := tool.InputSchema.Properties["path"]
	assert.True(t, ok)
	_, ok = tool.InputSchema.Properties["find"]
	assert.True(t, ok)
	_, ok = tool.InputSchema.Properties["replace"]
	assert.True(t, ok)
	_, ok = tool.InputSchema.Properties["dry_run"]
	assert.True(t, ok)
}

func TestReadFileSchema(t *testing.T) {
	fsserver, err := filesystemserver.NewFilesystemServer([]string{t.TempDir()})
	require.NoError(t, err)

	mcpClient := startTestClient(t, fsserver)

	tool := getTool(t, mcpClient, "read_file")
	require.NotNil(t, tool)

	_, ok := tool.InputSchema.Properties["path"]
	assert.True(t, ok)
}

// read_file must return raw bytes - tabs and CRLF preserved, no normalization
func TestReadFileRawBytes(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := tmpDir + "/tab_crlf.go"

	raw := "package main\r\n\tfunc main() {\r\n\t\tprintln(\"hello\")\r\n\t}\r\n"
	err := os.WriteFile(testFile, []byte(raw), 0644)
	require.NoError(t, err)

	fsserver, err := filesystemserver.NewFilesystemServer([]string{tmpDir})
	require.NoError(t, err)
	mcpClient := startTestClient(t, fsserver)

	result, err := mcpClient.CallTool(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "read_file",
			Arguments: map[string]interface{}{"path": testFile},
		},
	})
	require.NoError(t, err)
	require.False(t, result.IsError)

	var text string
	for _, c := range result.Content {
		if tc, ok := c.(mcp.TextContent); ok {
			text = tc.Text
			break
		}
	}

	assert.True(t, strings.Contains(text, "\t"), "read_file should preserve raw tab bytes, got: %q", text)
	assert.True(t, strings.Contains(text, "\r\n"), "read_file should preserve raw CRLF, got: %q", text)
	assert.Equal(t, raw, text, "read_file should return exact file content without normalization")
}
