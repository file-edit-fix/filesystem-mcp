package handler

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSearchWithinFiles_Found(t *testing.T) {
	dir := t.TempDir()

	// Create test files
	err := os.WriteFile(filepath.Join(dir, "file1.txt"), []byte("hello world\nfoo bar"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(dir, "file2.txt"), []byte("goodbye world\nhello again"), 0644)
	require.NoError(t, err)

	handler, err := NewFilesystemHandler(resolveAllowedDirs(t, dir))
	require.NoError(t, err)

	request := mcp.CallToolRequest{}
	request.Params.Name = "search_within_files"
	request.Params.Arguments = map[string]any{
		"path":      dir,
		"substring": "hello",
	}

	result, err := handler.HandleSearchWithinFiles(context.Background(), request)
	require.NoError(t, err)
	assert.False(t, result.IsError)

	output := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, output, "Found 2 occurrences")
	assert.Contains(t, output, "hello world")
	assert.Contains(t, output, "hello again")
}

func TestSearchWithinFiles_NoMatch(t *testing.T) {
	dir := t.TempDir()

	err := os.WriteFile(filepath.Join(dir, "file1.txt"), []byte("hello world"), 0644)
	require.NoError(t, err)

	handler, err := NewFilesystemHandler(resolveAllowedDirs(t, dir))
	require.NoError(t, err)

	request := mcp.CallToolRequest{}
	request.Params.Name = "search_within_files"
	request.Params.Arguments = map[string]any{
		"path":      dir,
		"substring": "zzz",
	}

	result, err := handler.HandleSearchWithinFiles(context.Background(), request)
	require.NoError(t, err)
	assert.False(t, result.IsError)

	output := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, output, "No occurrences")
}

func TestSearchWithinFiles_EmptySubstring(t *testing.T) {
	dir := t.TempDir()
	handler, err := NewFilesystemHandler(resolveAllowedDirs(t, dir))
	require.NoError(t, err)

	request := mcp.CallToolRequest{}
	request.Params.Name = "search_within_files"
	request.Params.Arguments = map[string]any{
		"path":      dir,
		"substring": "",
	}

	result, err := handler.HandleSearchWithinFiles(context.Background(), request)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].(mcp.TextContent).Text, "substring cannot be empty")
}

func TestSearchWithinFiles_NotADirectory(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "file.txt")
	err := os.WriteFile(filePath, []byte("hello"), 0644)
	require.NoError(t, err)

	handler, err := NewFilesystemHandler(resolveAllowedDirs(t, dir))
	require.NoError(t, err)

	request := mcp.CallToolRequest{}
	request.Params.Name = "search_within_files"
	request.Params.Arguments = map[string]any{
		"path":      filePath,
		"substring": "hello",
	}

	result, err := handler.HandleSearchWithinFiles(context.Background(), request)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].(mcp.TextContent).Text, "search path must be a directory")
}

func TestSearchWithinFiles_NegativeDepth(t *testing.T) {
	dir := t.TempDir()
	handler, err := NewFilesystemHandler(resolveAllowedDirs(t, dir))
	require.NoError(t, err)

	request := mcp.CallToolRequest{}
	request.Params.Name = "search_within_files"
	request.Params.Arguments = map[string]any{
		"path":      dir,
		"substring": "hello",
		"depth":     -1,
	}

	result, err := handler.HandleSearchWithinFiles(context.Background(), request)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].(mcp.TextContent).Text, "depth cannot be negative")
}

func TestSearchWithinFiles_NoAccess(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()

	handler, err := NewFilesystemHandler(resolveAllowedDirs(t, dir1))
	require.NoError(t, err)

	request := mcp.CallToolRequest{}
	request.Params.Name = "search_within_files"
	request.Params.Arguments = map[string]any{
		"path":      dir2,
		"substring": "hello",
	}

	result, err := handler.HandleSearchWithinFiles(context.Background(), request)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].(mcp.TextContent).Text, "access denied")
}

func TestSearchWithinFiles_MaxResultsLimit(t *testing.T) {
	dir := t.TempDir()

	// Create a file with many matching lines
	var content string
	for i := 0; i < 10; i++ {
		content += "match line\n"
	}
	err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte(content), 0644)
	require.NoError(t, err)

	handler, err := NewFilesystemHandler(resolveAllowedDirs(t, dir))
	require.NoError(t, err)

	request := mcp.CallToolRequest{}
	request.Params.Name = "search_within_files"
	request.Params.Arguments = map[string]any{
		"path":        dir,
		"substring":   "match",
		"max_results": float64(3),
	}

	result, err := handler.HandleSearchWithinFiles(context.Background(), request)
	require.NoError(t, err)
	assert.False(t, result.IsError)

	output := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, output, "Found 3 occurrences")
	assert.Contains(t, output, "Results limited to 3 matches")
}

func TestSearchWithinFiles_SubdirSearch(t *testing.T) {
	dir := t.TempDir()

	// Create nested files
	subdir := filepath.Join(dir, "subdir")
	err := os.MkdirAll(subdir, 0755)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(dir, "root.txt"), []byte("root content"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(subdir, "nested.txt"), []byte("nested content"), 0644)
	require.NoError(t, err)

	handler, err := NewFilesystemHandler(resolveAllowedDirs(t, dir))
	require.NoError(t, err)

	// Search entire tree for "content" - should find both
	request := mcp.CallToolRequest{}
	request.Params.Name = "search_within_files"
	request.Params.Arguments = map[string]any{
		"path":      dir,
		"substring": "content",
	}

	result, err := handler.HandleSearchWithinFiles(context.Background(), request)
	require.NoError(t, err)
	assert.False(t, result.IsError)

	output := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, output, "Found 2 occurrences")
}

func TestSearchWithinFiles_DepthLimit(t *testing.T) {
	dir := t.TempDir()

	// Create nested files deep
	deepDir := filepath.Join(dir, "level1", "level2")
	err := os.MkdirAll(deepDir, 0755)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(dir, "root.txt"), []byte("target"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(deepDir, "deep.txt"), []byte("target"), 0644)
	require.NoError(t, err)

	handler, err := NewFilesystemHandler(resolveAllowedDirs(t, dir))
	require.NoError(t, err)

	// Search with depth=1 (only root, not subdirs)
	request := mcp.CallToolRequest{}
	request.Params.Name = "search_within_files"
	request.Params.Arguments = map[string]any{
		"path":      dir,
		"substring": "target",
		"depth":     float64(1),
	}

	result, err := handler.HandleSearchWithinFiles(context.Background(), request)
	require.NoError(t, err)
	assert.False(t, result.IsError)

	output := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, output, "Found 1 occurrence")
	assert.Contains(t, output, "root.txt")
}