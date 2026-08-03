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

func TestModifyFile_FuzzyMatch_IndentDifference(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.go")
	originalContent := "package main\n\n\tfunc main() {\n\t\tx := 1\n\t\treturn x\n\t}\n"
	err := os.WriteFile(filePath, []byte(originalContent), 0644)
	require.NoError(t, err)

	handler, err := NewFilesystemHandler(resolveAllowedDirs(t, dir))
	require.NoError(t, err)

	// find uses spaces instead of tabs — exact match fails,
	// fuzzy should match by content similarity after normalizing indent
	request := mcp.CallToolRequest{}
	request.Params.Name = "modify_file"
	request.Params.Arguments = map[string]any{
		"path":            filePath,
		"find":            "x := 1\nreturn x",
		"replace":         "y := 2\nreturn y",
		"all_occurrences": false,
	}

	result, err := handler.HandleModifyFile(context.Background(), request)
	require.NoError(t, err)
	assert.False(t, result.IsError)

	content, err := os.ReadFile(filePath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "y := 2")
	assert.Contains(t, string(content), "return y")
	assert.NotContains(t, string(content), "x := 1")
}

func TestModifyFile_FuzzyMatch_NoMatch(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.txt")
	originalContent := "hello world\nfoo bar\nbaz qux\n"
	err := os.WriteFile(filePath, []byte(originalContent), 0644)
	require.NoError(t, err)

	handler, err := NewFilesystemHandler(resolveAllowedDirs(t, dir))
	require.NoError(t, err)

	// find with completely unrelated content — should not modify
	request := mcp.CallToolRequest{}
	request.Params.Name = "modify_file"
	request.Params.Arguments = map[string]any{
		"path":            filePath,
		"find":            "nothing matches here at all",
		"replace":         "replacement",
		"all_occurrences": false,
	}

	result, err := handler.HandleModifyFile(context.Background(), request)
	require.NoError(t, err)
	assert.False(t, result.IsError)

	content, err := os.ReadFile(filePath)
	require.NoError(t, err)
	assert.Equal(t, originalContent, string(content))
}

func TestModifyFile_FuzzyMatch_MultiLineBlock(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.go")
	originalContent := "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n"
	err := os.WriteFile(filePath, []byte(originalContent), 0644)
	require.NoError(t, err)

	handler, err := NewFilesystemHandler(resolveAllowedDirs(t, dir))
	require.NoError(t, err)

	// find uses spaces, file uses tabs — fuzzy match should find the function body
	request := mcp.CallToolRequest{}
	request.Params.Name = "modify_file"
	request.Params.Arguments = map[string]any{
		"path":            filePath,
		"find":            "fmt.Println(\"hello\")",
		"replace":         "fmt.Println(\"world\")",
		"all_occurrences": false,
	}

	result, err := handler.HandleModifyFile(context.Background(), request)
	require.NoError(t, err)
	assert.False(t, result.IsError)

	content, err := os.ReadFile(filePath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "world")
	assert.NotContains(t, string(content), "hello")
}
func TestModifyFile_RegexReplaceWithEscapedNewline(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.txt")
	originalContent := "item1, item2, item3"
	err := os.WriteFile(filePath, []byte(originalContent), 0644)
	require.NoError(t, err)

	handler, err := NewFilesystemHandler(resolveAllowedDirs(t, dir))
	require.NoError(t, err)

	// Use \n (backslash+n, 0x5c 0x6e) in replace — this is what JSON "\n" becomes
	// interpretEscapeSequences should convert \n to actual newline (0x0a)
	request := mcp.CallToolRequest{}
	request.Params.Name = "modify_file"
	request.Params.Arguments = map[string]any{
		"path":            filePath,
		"find":            ", ",
		"replace":         "\\n",
		"regex":           true,
		"all_occurrences": true,
	}

	result, err := handler.HandleModifyFile(context.Background(), request)
	require.NoError(t, err)
	assert.False(t, result.IsError)

	content, err := os.ReadFile(filePath)
	require.NoError(t, err)
	assert.Equal(t, "item1\nitem2\nitem3", string(content),
		"backslash+n in replace should be interpreted as actual newline")
}

func TestModifyFile_ExactReplaceWithEscapedNewline(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.txt")
	originalContent := "item1, item2, item3"
	err := os.WriteFile(filePath, []byte(originalContent), 0644)
	require.NoError(t, err)

	handler, err := NewFilesystemHandler(resolveAllowedDirs(t, dir))
	require.NoError(t, err)

	// Non-regex (exact match) path: \n (backslash+n) in replace should be
	// interpreted as actual newline via interpretEscapeSequences
	request := mcp.CallToolRequest{}
	request.Params.Name = "modify_file"
	request.Params.Arguments = map[string]any{
		"path":            filePath,
		"find":            ", ",
		"replace":         "\\n",
		"regex":           false,
		"all_occurrences": true,
	}

	result, err := handler.HandleModifyFile(context.Background(), request)
	require.NoError(t, err)
	assert.False(t, result.IsError)

	content, err := os.ReadFile(filePath)
	require.NoError(t, err)
	assert.Equal(t, "item1\nitem2\nitem3", string(content),
		"non-regex: backslash+n in replace should be interpreted as actual newline")
}

func TestModifyFile_ExactReplaceSingleWithEscapedNewline(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.txt")
	originalContent := "item1, item2, item3"
	err := os.WriteFile(filePath, []byte(originalContent), 0644)
	require.NoError(t, err)

	handler, err := NewFilesystemHandler(resolveAllowedDirs(t, dir))
	require.NoError(t, err)

	// Non-regex, single replacement (all_occurrences=false) with \n in replace
	request := mcp.CallToolRequest{}
	request.Params.Name = "modify_file"
	request.Params.Arguments = map[string]any{
		"path":            filePath,
		"find":            ", ",
		"replace":         "\\n",
		"regex":           false,
		"all_occurrences": false,
	}

	result, err := handler.HandleModifyFile(context.Background(), request)
	require.NoError(t, err)
	assert.False(t, result.IsError)

	// Only first occurrence replaced with newline
	content, err := os.ReadFile(filePath)
	require.NoError(t, err)
	assert.Equal(t, "item1\nitem2, item3", string(content),
		"non-regex single: first comma replaced with newline, second unchanged")
}

func TestModifyFile_ExactReplaceWithEscapedTab(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.txt")
	originalContent := "before match after"
	err := os.WriteFile(filePath, []byte(originalContent), 0644)
	require.NoError(t, err)

	handler, err := NewFilesystemHandler(resolveAllowedDirs(t, dir))
	require.NoError(t, err)

	// Non-regex path: \t in replace should be interpreted as actual tab
	request := mcp.CallToolRequest{}
	request.Params.Name = "modify_file"
	request.Params.Arguments = map[string]any{
		"path":            filePath,
		"find":            "match",
		"replace":         "replaced\twith\ttab",
		"regex":           false,
		"all_occurrences": true,
	}

	result, err := handler.HandleModifyFile(context.Background(), request)
	require.NoError(t, err)
	assert.False(t, result.IsError)

	content, err := os.ReadFile(filePath)
	require.NoError(t, err)
	assert.Equal(t, "before replaced\twith\ttab after", string(content))
}

func TestModifyFile_ExactReplaceWithEscapedBackslash(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.txt")
	originalContent := "path: C:\\Users\\name"
	err := os.WriteFile(filePath, []byte(originalContent), 0644)
	require.NoError(t, err)

	handler, err := NewFilesystemHandler(resolveAllowedDirs(t, dir))
	require.NoError(t, err)

	// Non-regex path: \\ in replace should be interpreted as literal backslash
	// The JSON value is "\\\\" → Go string is "\\" → interpretEscapeSequences converts to "\"
	// In exact match mode, find uses the file's literal backslash content: "C:\Users\name"
	request := mcp.CallToolRequest{}
	request.Params.Name = "modify_file"
	request.Params.Arguments = map[string]any{
		"path":            filePath,
		"find":            "C:\\Users\\name",
		"replace":         "D:\\\\Users\\\\newuser",
		"regex":           false,
		"all_occurrences": true,
	}

	result, err := handler.HandleModifyFile(context.Background(), request)
	require.NoError(t, err)
	assert.False(t, result.IsError)

	content, err := os.ReadFile(filePath)
	require.NoError(t, err)
	assert.Equal(t, "path: D:\\Users\\newuser", string(content))
}
// TestModifyFile_CRLF_MatchNewlineInFind tests that backslash-n in find matches
// backslash-r-backslash-n in the file when using exact match mode (CRLF normalization).
func TestModifyFile_CRLF_MatchNewlineInFind(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.txt")
	// Create a file with CRLF line endings
	originalContent := "foo\r\nbar\r\nbaz"
	err := os.WriteFile(filePath, []byte(originalContent), 0644)
	require.NoError(t, err)

	handler, err := NewFilesystemHandler(resolveAllowedDirs(t, dir))
	require.NoError(t, err)

	// Find: "\n" (single newline character 0x0a) — should match CRLF after normalization
	request := mcp.CallToolRequest{}
	request.Params.Name = "modify_file"
	request.Params.Arguments = map[string]any{
		"path":            filePath,
		"find":            "\n",
		"replace":         "X",
		"all_occurrences": true,
	}

	result, err := handler.HandleModifyFile(context.Background(), request)
	require.NoError(t, err)
	assert.False(t, result.IsError)

	content, err := os.ReadFile(filePath)
	require.NoError(t, err)
	assert.Equal(t, "fooXbarXbaz", string(content),
		"backslash-n in find should match CRLF in file after normalization")
}

func TestModifyFile_CRLF_MatchNewlineInFind_MultiLine(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.txt")
	originalContent := "line1\r\nline2\r\nline3"
	err := os.WriteFile(filePath, []byte(originalContent), 0644)
	require.NoError(t, err)

	handler, err := NewFilesystemHandler(resolveAllowedDirs(t, dir))
	require.NoError(t, err)

	request := mcp.CallToolRequest{}
	request.Params.Name = "modify_file"
	request.Params.Arguments = map[string]any{
		"path":            filePath,
		"find":            "line1\nline2",
		"replace":         "REPLACED",
		"all_occurrences": true,
	}

	result, err := handler.HandleModifyFile(context.Background(), request)
	require.NoError(t, err)
	assert.False(t, result.IsError)

	content, err := os.ReadFile(filePath)
	require.NoError(t, err)
	assert.Equal(t, "REPLACED\nline3", string(content),
		"multi-line find with backslash-n should match across CRLF boundary")
}

func TestModifyFile_CRLF_MatchFindWithCRLF(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.txt")
	originalContent := "line1\r\nline2\r\nline3"
	err := os.WriteFile(filePath, []byte(originalContent), 0644)
	require.NoError(t, err)

	handler, err := NewFilesystemHandler(resolveAllowedDirs(t, dir))
	require.NoError(t, err)

	request := mcp.CallToolRequest{}
	request.Params.Name = "modify_file"
	request.Params.Arguments = map[string]any{
		"path":            filePath,
		"find":            "line1\r\nline2",
		"replace":         "REPLACED",
		"all_occurrences": true,
	}

	result, err := handler.HandleModifyFile(context.Background(), request)
	require.NoError(t, err)
	assert.False(t, result.IsError)

	content, err := os.ReadFile(filePath)
	require.NoError(t, err)
	assert.Equal(t, "REPLACED\nline3", string(content),
		"find with backslash-r-backslash-n should match CRLF file after normalization")
}

func TestModifyFile_DryRun_NoModification(t *testing.T) {
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
		"dry_run":         true,
	}

	result, err := handler.HandleModifyFile(context.Background(), request)
	require.NoError(t, err)
	assert.False(t, result.IsError)

	// File should be unchanged
	content, err := os.ReadFile(filePath)
	require.NoError(t, err)
	assert.Equal(t, originalContent, string(content))

	// Response should mention dry run
	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "Dry run")
	assert.Contains(t, text, "match(es) found")
}

func TestModifyFile_DryRun_LineTrimFallback(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.go")
	originalContent := "package main\n\n\tfunc main() {\n\t\tx := 1\n\t\treturn x\n\t}\n"
	err := os.WriteFile(filePath, []byte(originalContent), 0644)
	require.NoError(t, err)

	handler, err := NewFilesystemHandler(resolveAllowedDirs(t, dir))
	require.NoError(t, err)

	// find uses spaces instead of tabs — should match via line-level trim fallback
	request := mcp.CallToolRequest{}
	request.Params.Name = "modify_file"
	request.Params.Arguments = map[string]any{
		"path":            filePath,
		"find":            "x := 1\nreturn x",
		"replace":         "y := 2\nreturn y",
		"all_occurrences": false,
		"dry_run":         true,
	}

	result, err := handler.HandleModifyFile(context.Background(), request)
	require.NoError(t, err)
	assert.False(t, result.IsError)

	// File should be unchanged
	content, err := os.ReadFile(filePath)
	require.NoError(t, err)
	assert.Equal(t, originalContent, string(content))

	// Response should show the match
	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "x := 1")
	assert.Contains(t, text, "return x")
}

func TestModifyFile_MixedMatch_ExactThenTrim(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.go")
	originalContent := "\tfunc foo() {\n\t\tx := 1\n\t}\n\tfunc bar() {\n\t\tx := 2\n\t}\n"
	err := os.WriteFile(filePath, []byte(originalContent), 0644)
	require.NoError(t, err)

	handler, err := NewFilesystemHandler(resolveAllowedDirs(t, dir))
	require.NoError(t, err)

	// find has leading "\t" that doesn't match file content exactly;
	// trim fallback matches on first occurrence (all_occurrences: false)
	request := mcp.CallToolRequest{}
	request.Params.Name = "modify_file"
	request.Params.Arguments = map[string]any{
		"path":            filePath,
		"find":            "x := 1\n}",
		"replace":         "y := 99\n}",
		"all_occurrences": false,
	}

	result, err := handler.HandleModifyFile(context.Background(), request)
	require.NoError(t, err)
	assert.False(t, result.IsError)

	content, err := os.ReadFile(filePath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "y := 99")
	assert.NotContains(t, string(content), "x := 1")
	// Second occurrence unchanged
	assert.Contains(t, string(content), "x := 2")
}

func TestModifyFile_DryRun_NoMatch(t *testing.T) {
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
		"path":     filePath,
		"find":     "zzz",
		"replace":  "aaa",
		"dry_run":  true,
	}

	result, err := handler.HandleModifyFile(context.Background(), request)
	require.NoError(t, err)
	assert.False(t, result.IsError)

	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "No matches found")
}

func TestModifyFile_TrimFallback_FindLongerThanFile(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.txt")
	originalContent := "hello world"
	err := os.WriteFile(filePath, []byte(originalContent), 0644)
	require.NoError(t, err)

	handler, err := NewFilesystemHandler(resolveAllowedDirs(t, dir))
	require.NoError(t, err)

	// Multi-line find against a single-line file — should not panic
	request := mcp.CallToolRequest{}
	request.Params.Name = "modify_file"
	request.Params.Arguments = map[string]any{
		"path":            filePath,
		"find":            "line1\nline2\nline3",
		"replace":         "replaced",
		"all_occurrences": false,
	}

	result, err := handler.HandleModifyFile(context.Background(), request)
	require.NoError(t, err)
	assert.False(t, result.IsError)

	content, err := os.ReadFile(filePath)
	require.NoError(t, err)
	assert.Equal(t, originalContent, string(content), "file should be unchanged when find is longer than file")
}

func TestModifyFile_AtomicWrite_PreservesPermissions(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.txt")
	originalContent := "hello world"
	err := os.WriteFile(filePath, []byte(originalContent), 0644)
	require.NoError(t, err)

	originalInfo, err := os.Stat(filePath)
	require.NoError(t, err)
	originalMode := originalInfo.Mode().Perm()

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

	info, err := os.Stat(filePath)
	require.NoError(t, err)
	assert.Equal(t, originalMode, info.Mode().Perm(), "file permissions should be preserved after atomic write")
}

func TestModifyFile_TrimFallback_EmptyLineDoesNotMatchIndent(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.go")
	originalContent := "func foo() {\n\t\t\n\t\tx := 1\n\t}\n"
	err := os.WriteFile(filePath, []byte(originalContent), 0644)
	require.NoError(t, err)

	handler, err := NewFilesystemHandler(resolveAllowedDirs(t, dir))
	require.NoError(t, err)

	// find has an empty line where file has a whitespace-only line — should NOT match
	request := mcp.CallToolRequest{}
	request.Params.Name = "modify_file"
	request.Params.Arguments = map[string]any{
		"path":            filePath,
		"find":            "func foo() {\n\n\t\tx := 1\n\t}\n",
		"replace":         "func bar() {\n\n\t\tx := 99\n\t}\n",
		"all_occurrences": false,
	}

	result, err := handler.HandleModifyFile(context.Background(), request)
	require.NoError(t, err)
	assert.False(t, result.IsError)

	content, err := os.ReadFile(filePath)
	require.NoError(t, err)
	assert.Equal(t, originalContent, string(content), "empty line in find should not match whitespace-only line in file")
}

func TestModifyFile_EmptyFind_NoMatch(t *testing.T) {
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
		"find":            "",
		"replace":         "X",
		"all_occurrences": true,
	}

	result, err := handler.HandleModifyFile(context.Background(), request)
	require.NoError(t, err)
	assert.False(t, result.IsError)

	content, err := os.ReadFile(filePath)
	require.NoError(t, err)
	assert.Equal(t, originalContent, string(content), "empty find should not modify the file")
}

func TestModifyFile_OverlappingMatch(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.txt")
	originalContent := "aaaaa"
	err := os.WriteFile(filePath, []byte(originalContent), 0644)
	require.NoError(t, err)

	handler, err := NewFilesystemHandler(resolveAllowedDirs(t, dir))
	require.NoError(t, err)

	request := mcp.CallToolRequest{}
	request.Params.Name = "modify_file"
	request.Params.Arguments = map[string]any{
		"path":            filePath,
		"find":            "aa",
		"replace":         "X",
		"all_occurrences": true,
	}

	result, err := handler.HandleModifyFile(context.Background(), request)
	require.NoError(t, err)
	assert.False(t, result.IsError)

	content, err := os.ReadFile(filePath)
	require.NoError(t, err)
	assert.Equal(t, "XXa", string(content),
		"'aaaaa' with find='aa' uses non-overlapping replacement: XXa")
}
