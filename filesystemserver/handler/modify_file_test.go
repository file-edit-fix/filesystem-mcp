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

func TestModifyFile_BasicReplace(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.txt")
	originalContent := "hello world hello"
	err := os.WriteFile(filePath, []byte(originalContent), 0644)
	require.NoError(t, err)

	handler, err := NewFilesystemHandler(resolveAllowedDirs(t, dir))
	require.NoError(t, err)

	request := mcp.CallToolRequest{}
	request.Params.Name = "modify_file"
	request.Params.Arguments = map[string]any{
		"path":            filePath,
		"find":            "hello",
		"replace":         "hi",
		"all_occurrences": true,
	}

	result, err := handler.HandleModifyFile(context.Background(), request)
	require.NoError(t, err)
	assert.False(t, result.IsError)

	// Verify file content
	content, err := os.ReadFile(filePath)
	require.NoError(t, err)
	assert.Equal(t, "hi world hi", string(content))
}

func TestModifyFile_SingleReplace(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.txt")
	originalContent := "hello world hello"
	err := os.WriteFile(filePath, []byte(originalContent), 0644)
	require.NoError(t, err)

	handler, err := NewFilesystemHandler(resolveAllowedDirs(t, dir))
	require.NoError(t, err)

	request := mcp.CallToolRequest{}
	request.Params.Name = "modify_file"
	request.Params.Arguments = map[string]any{
		"path":            filePath,
		"find":            "hello",
		"replace":         "hi",
		"all_occurrences": false,
	}

	result, err := handler.HandleModifyFile(context.Background(), request)
	require.NoError(t, err)
	assert.False(t, result.IsError)

	// Only first occurrence should be replaced
	content, err := os.ReadFile(filePath)
	require.NoError(t, err)
	assert.Equal(t, "hi world hello", string(content))
}

func TestModifyFile_RegexReplace(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.txt")
	originalContent := "foo123 bar456 baz789"
	err := os.WriteFile(filePath, []byte(originalContent), 0644)
	require.NoError(t, err)

	handler, err := NewFilesystemHandler(resolveAllowedDirs(t, dir))
	require.NoError(t, err)

	request := mcp.CallToolRequest{}
	request.Params.Name = "modify_file"
	request.Params.Arguments = map[string]any{
		"path":            filePath,
		"find":            "[0-9]+",
		"replace":         "NUM",
		"regex":           true,
		"all_occurrences": true,
	}

	result, err := handler.HandleModifyFile(context.Background(), request)
	require.NoError(t, err)
	assert.False(t, result.IsError)

	content, err := os.ReadFile(filePath)
	require.NoError(t, err)
	assert.Equal(t, "fooNUM barNUM bazNUM", string(content))
}

func TestModifyFile_FileNotFound(t *testing.T) {
	dir := t.TempDir()
	handler, err := NewFilesystemHandler(resolveAllowedDirs(t, dir))
	require.NoError(t, err)

	request := mcp.CallToolRequest{}
	request.Params.Name = "modify_file"
	request.Params.Arguments = map[string]any{
		"path":    filepath.Join(dir, "nonexistent.txt"),
		"find":    "hello",
		"replace": "hi",
	}

	result, err := handler.HandleModifyFile(context.Background(), request)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].(mcp.TextContent).Text, "File not found")
}

func TestModifyFile_InvalidRegex(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.txt")
	err := os.WriteFile(filePath, []byte("hello"), 0644)
	require.NoError(t, err)

	handler, err := NewFilesystemHandler(resolveAllowedDirs(t, dir))
	require.NoError(t, err)

	request := mcp.CallToolRequest{}
	request.Params.Name = "modify_file"
	request.Params.Arguments = map[string]any{
		"path":  filePath,
		"find":  "[invalid",
		"replace": "x",
		"regex": true,
	}

	result, err := handler.HandleModifyFile(context.Background(), request)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].(mcp.TextContent).Text, "Invalid regular expression")
}

func TestModifyFile_DirectoryError(t *testing.T) {
	dir := t.TempDir()
	handler, err := NewFilesystemHandler(resolveAllowedDirs(t, dir))
	require.NoError(t, err)

	request := mcp.CallToolRequest{}
	request.Params.Name = "modify_file"
	request.Params.Arguments = map[string]any{
		"path":    dir,
		"find":    "hello",
		"replace": "hi",
	}

	result, err := handler.HandleModifyFile(context.Background(), request)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].(mcp.TextContent).Text, "Cannot modify a directory")
}

func TestModifyFile_NoAccess(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()

	handler, err := NewFilesystemHandler(resolveAllowedDirs(t, dir1))
	require.NoError(t, err)

	request := mcp.CallToolRequest{}
	request.Params.Name = "modify_file"
	request.Params.Arguments = map[string]any{
		"path":    filepath.Join(dir2, "test.txt"),
		"find":    "hello",
		"replace": "hi",
	}

	result, err := handler.HandleModifyFile(context.Background(), request)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].(mcp.TextContent).Text, "access denied")
}

func TestModifyFile_NoMatch(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.txt")
	originalContent := "hello world"
	err := os.WriteFile(filePath, []byte(originalContent), 0644)
	require.NoError(t, err)

	handler, err := NewFilesystemHandler(resolveAllowedDirs(t, dir))
	require.NoError(t, err)

	request := mcp.CallToolRequest{}
	request.Params.Name = "modify_file"
	request.Params.Arguments = map[string]any{
		"path":            filePath,
		"find":            "zzz",
		"replace":         "aaa",
		"all_occurrences": true,
	}

	result, err := handler.HandleModifyFile(context.Background(), request)
	require.NoError(t, err)
	assert.False(t, result.IsError)

	// Content should be unchanged
	content, err := os.ReadFile(filePath)
	require.NoError(t, err)
	assert.Equal(t, originalContent, string(content))
}

func TestModifyFile_CRLF_ExactMatch(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.txt")
	// Create a file with CRLF line endings
	originalContent := "hello world\r\nfoo bar\r\nbaz qux"
	err := os.WriteFile(filePath, []byte(originalContent), 0644)
	require.NoError(t, err)

	handler, err := NewFilesystemHandler(resolveAllowedDirs(t, dir))
	require.NoError(t, err)

	request := mcp.CallToolRequest{}
	request.Params.Name = "modify_file"
	request.Params.Arguments = map[string]any{
		"path":            filePath,
		"find":            "foo bar",
		"replace":         "replaced",
		"all_occurrences": true,
	}

	result, err := handler.HandleModifyFile(context.Background(), request)
	require.NoError(t, err)
	assert.False(t, result.IsError)

	// Verify file content — CRLF normalization means output is LF
	content, err := os.ReadFile(filePath)
	require.NoError(t, err)
	assert.Equal(t, "hello world\nreplaced\nbaz qux", string(content))
}

func TestModifyFile_CRLF_MultiLineExactMatch(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.txt")
	// Create a file with CRLF line endings
	originalContent := "line1\r\nline2\r\nline3"
	err := os.WriteFile(filePath, []byte(originalContent), 0644)
	require.NoError(t, err)

	handler, err := NewFilesystemHandler(resolveAllowedDirs(t, dir))
	require.NoError(t, err)

	request := mcp.CallToolRequest{}
	request.Params.Name = "modify_file"
	request.Params.Arguments = map[string]any{
		"path":            filePath,
		"find":            "line2",
		"replace":         "modified",
		"all_occurrences": true,
	}

	result, err := handler.HandleModifyFile(context.Background(), request)
	require.NoError(t, err)
	assert.False(t, result.IsError)

	content, err := os.ReadFile(filePath)
	require.NoError(t, err)
	assert.Equal(t, "line1\nmodified\nline3", string(content))
}

func TestModifyFile_RegexReplaceEscapeSequences(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.txt")
	originalContent := "before match after"
	err := os.WriteFile(filePath, []byte(originalContent), 0644)
	require.NoError(t, err)

	handler, err := NewFilesystemHandler(resolveAllowedDirs(t, dir))
	require.NoError(t, err)

	// Use \t (tab) in regex replace — should be interpreted as actual tab
	request := mcp.CallToolRequest{}
	request.Params.Name = "modify_file"
	request.Params.Arguments = map[string]any{
		"path":            filePath,
		"find":            "match",
		"replace":         "replaced\twith\ttab",
		"regex":           true,
		"all_occurrences": true,
	}

	result, err := handler.HandleModifyFile(context.Background(), request)
	require.NoError(t, err)
	assert.False(t, result.IsError)

	content, err := os.ReadFile(filePath)
	require.NoError(t, err)
	assert.Equal(t, "before replaced\twith\ttab after", string(content))
}

func TestModifyFile_RegexReplaceNewline(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.txt")
	originalContent := "item1, item2, item3"
	err := os.WriteFile(filePath, []byte(originalContent), 0644)
	require.NoError(t, err)

	handler, err := NewFilesystemHandler(resolveAllowedDirs(t, dir))
	require.NoError(t, err)

	// Use \n (newline) in regex replace — should be interpreted as actual newline
	request := mcp.CallToolRequest{}
	request.Params.Name = "modify_file"
	request.Params.Arguments = map[string]any{
		"path":            filePath,
		"find":            ", ",
		"replace":         "\n",
		"regex":           true,
		"all_occurrences": true,
	}

	result, err := handler.HandleModifyFile(context.Background(), request)
	require.NoError(t, err)
	assert.False(t, result.IsError)

	content, err := os.ReadFile(filePath)
	require.NoError(t, err)
	assert.Equal(t, "item1\nitem2\nitem3", string(content))
}

func TestModifyFile_RegexReplaceBackslashLiteral(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.txt")
	originalContent := "path: C:\\Users\\name"
	err := os.WriteFile(filePath, []byte(originalContent), 0644)
	require.NoError(t, err)

	handler, err := NewFilesystemHandler(resolveAllowedDirs(t, dir))
	require.NoError(t, err)

	// Use \\\\ (double escape) in regex replace — should be interpreted as literal backslash
	// The JSON value is "\\\\" → Go string is "\\" → interpretEscapeSequences converts to "\"
	request := mcp.CallToolRequest{}
	request.Params.Name = "modify_file"
	request.Params.Arguments = map[string]any{
		"path":            filePath,
		"find":            "C:\\\\Users\\\\name",
		"replace":         "D:\\\\Users\\\\newuser",
		"regex":           true,
		"all_occurrences": true,
	}

	result, err := handler.HandleModifyFile(context.Background(), request)
	require.NoError(t, err)
	assert.False(t, result.IsError)

	content, err := os.ReadFile(filePath)
	require.NoError(t, err)
	assert.Equal(t, "path: D:\\Users\\newuser", string(content))
}