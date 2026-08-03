package handler

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// matchResult holds the outcome of a matching attempt using byte offsets.
type matchResult struct {
	startByte   int    // byte offset where match starts in content
	endByte     int    // byte offset where match ends (exclusive)
	replacement string // the replacement text to inject
}

// HandleModifyFile handles the modify_file tool request
func (fs *FilesystemHandler) HandleModifyFile(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	path, err := request.RequireString("path")
	if err != nil {
		return nil, err
	}

	find, err := request.RequireString("find")
	if err != nil {
		return nil, err
	}

	replace, err := request.RequireString("replace")
	if err != nil {
		return nil, err
	}

	allOccurrences := true
	if val, err := request.RequireBool("all_occurrences"); err == nil {
		allOccurrences = val
	}

	useRegex := false
	if val, err := request.RequireBool("regex"); err == nil {
		useRegex = val
	}

	dryRun := false
	if val, err := request.RequireBool("dry_run"); err == nil {
		dryRun = val
	}

	resolvedPath, err := resolvePath(path)
	if err != nil {
		return errorResult(fmt.Sprintf("Error: %v", err)), nil
	}
	path = resolvedPath

	validPath, err := fs.validatePath(path)
	if err != nil {
		return errorResult(fmt.Sprintf("Error: %v", err)), nil
	}

	if info, err := os.Stat(validPath); err == nil && info.IsDir() {
		return errorResult("Error: Cannot modify a directory"), nil
	}

	if _, err := os.Stat(validPath); os.IsNotExist(err) {
		return errorResult(fmt.Sprintf("Error: File not found: %s", path)), nil
	}

	content, err := os.ReadFile(validPath)
	if err != nil {
		return errorResult(fmt.Sprintf("Error reading file: %v", err)), nil
	}

	// Normalize line endings once — all matching and replacement operates on this
	contentStr := strings.ReplaceAll(string(content), "\r\n", "\n")

	var replacements []matchResult

	if useRegex {
		replacements, err = regexReplace(contentStr, find, replace, allOccurrences)
	} else {
		replacements, err = mixedReplace(contentStr, find, replace, allOccurrences)
	}

	if err != nil {
		return errorResult(fmt.Sprintf("Error: Invalid regular expression: %v", err)), nil
	}

	if len(replacements) == 0 {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: "No matches found. File unchanged.",
				},
			},
		}, nil
	}

	if dryRun {
		return dryRunResult(contentStr, replacements), nil
	}

	modifiedContent := applyReplacements(contentStr, replacements)
	if err := atomicWriteFile(validPath, modifiedContent); err != nil {
		return errorResult(fmt.Sprintf("Error writing to file: %v", err)), nil
	}

	return successResult(path, validPath, replacements), nil
}

// mixedReplace performs replacement with mixed matching strategy:
// exact match first, then line-level trim fallback.
func mixedReplace(content, find, replace string, allOccurrences bool) ([]matchResult, error) {
	normalizedFind := strings.ReplaceAll(find, "\r\n", "\n")
	normalizedReplace := interpretEscapeSequences(replace)

	// Try exact match first
	exactCount := strings.Count(content, normalizedFind)
	if exactCount > 0 {
		matches := make([]matchResult, exactCount)
		offset := 0
		for i := 0; i < exactCount; i++ {
			idx := strings.Index(content[offset:], normalizedFind)
			startByte := offset + idx
			endByte := startByte + len(normalizedFind)
			matches[i] = matchResult{
				startByte:   startByte,
				endByte:     endByte,
				replacement: normalizedReplace,
			}
			offset = endByte
		}
		if allOccurrences {
			return matches, nil
		}
		return matches[:1], nil
	}

	// Exact match failed — try line-level trim fallback
	findLines := strings.Split(normalizedFind, "\n")
	contentLines := strings.Split(content, "\n")

	if len(findLines) > len(contentLines) {
		return nil, nil
	}

	for i := 0; i <= len(contentLines)-len(findLines); i++ {
		window := contentLines[i : i+len(findLines)]
		if linesTrimMatch(findLines, window) {
			startByte := lineOffset(content, i)
			endByte := lineOffset(content, i+len(findLines))
			if allOccurrences {
				var allMatches []matchResult
				searchStart := 0
				for {
					found := false
					for j := searchStart; j <= len(contentLines)-len(findLines); j++ {
						w := contentLines[j : j+len(findLines)]
						if linesTrimMatch(findLines, w) {
							sb := lineOffset(content, j)
							eb := lineOffset(content, j+len(findLines))
							allMatches = append(allMatches, matchResult{
								startByte:   sb,
								endByte:     eb,
								replacement: normalizedReplace,
							})
							searchStart = j + len(findLines)
							found = true
							break
						}
					}
					if !found {
						break
					}
				}
				return allMatches, nil
			}

			return []matchResult{{
				startByte:   startByte,
				endByte:     endByte,
				replacement: normalizedReplace,
			}}, nil
		}
	}

	return nil, nil
}

// linesTrimMatch returns true if every line in findLines matches the
// corresponding line in contentLines when both are trimmed of leading whitespace.
func linesTrimMatch(findLines, contentLines []string) bool {
	if len(findLines) > len(contentLines) {
		return false
	}
	for i, fl := range findLines {
		if strings.TrimLeft(fl, "\t ") != strings.TrimLeft(contentLines[i], "\t ") {
			return false
		}
	}
	return true
}

// regexReplace performs replacement using regex patterns.
func regexReplace(content, find, replace string, allOccurrences bool) ([]matchResult, error) {
	re, err := regexp.Compile(find)
	if err != nil {
		return nil, fmt.Errorf("invalid regular expression: %v", err)
	}

	normalizedReplace := interpretEscapeSequences(replace)

	if allOccurrences {
		locs := re.FindAllStringIndex(content, -1)
		if len(locs) == 0 {
			return nil, nil
		}
		matches := make([]matchResult, len(locs))
		for i, l := range locs {
			matches[i] = matchResult{
				startByte:   l[0],
				endByte:     l[1],
				replacement: normalizedReplace,
			}
		}
		return matches, nil
	}

	loc := re.FindStringIndex(content)
	if loc == nil {
		return nil, nil
	}

	return []matchResult{{
		startByte:   loc[0],
		endByte:     loc[1],
		replacement: normalizedReplace,
	}}, nil
}

// applyReplacements applies matchResults to content using byte offsets.
func applyReplacements(content string, matches []matchResult) string {
	var result strings.Builder
	result.WriteString(content[:matches[0].startByte])

	for i := 0; i < len(matches); i++ {
		result.WriteString(matches[i].replacement)
		if i+1 < len(matches) {
			result.WriteString(content[matches[i].endByte:matches[i+1].startByte])
		}
	}
	result.WriteString(content[matches[len(matches)-1].endByte:])

	return result.String()
}

// lineOffset converts a line index to a byte offset in the full text.
func lineOffset(text string, lineIndex int) int {
	lines := strings.Split(text, "\n")
	offset := 0
	for i := 0; i < lineIndex && i < len(lines); i++ {
		offset += len(lines[i]) + 1
	}
	return offset
}

// atomicWriteFile writes content to a temp file with the original file's
// permissions, then renames it atomically.
func atomicWriteFile(path, content string) error {
	info, err := os.Stat(path)
	var perm os.FileMode = 0644
	if err == nil {
		perm = info.Mode().Perm()
	}

	dir := filepath.Dir(path)
	tmpFile, err := os.CreateTemp(dir, "modify_file_*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %v", err)
	}
	tmpPath := tmpFile.Name()

	if _, err := tmpFile.WriteString(content); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("write temp file: %v", err)
	}
	if err := tmpFile.Chmod(perm); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("chmod temp file: %v", err)
	}
	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("close temp file: %v", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename temp file: %v", err)
	}

	return nil
}

// dryRunResult builds a dry_run response listing matched lines.
func dryRunResult(originalContent string, matches []matchResult) *mcp.CallToolResult {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Dry run: %d match(es) found (no changes applied)\n\n", len(matches)))

	for i, m := range matches {
		startLine := strings.Count(originalContent[:m.startByte], "\n")
		sb.WriteString(fmt.Sprintf("Match %d at line %d:\n", i+1, startLine+1))
		for _, line := range strings.Split(originalContent[m.startByte:m.endByte], "\n") {
			sb.WriteString(fmt.Sprintf("  %s\n", line))
		}
		sb.WriteString("  -> would be replaced with:\n")
		for _, rl := range strings.Split(m.replacement, "\n") {
			sb.WriteString(fmt.Sprintf("    %s\n", rl))
		}
		sb.WriteString("\n")
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: sb.String(),
			},
		},
	}
}

// successResult builds a success response.
func successResult(originalPath, validPath string, matches []matchResult) *mcp.CallToolResult {
	resourceURI := pathToResourceURI(validPath)

	info, err := os.Stat(validPath)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("File modified successfully. Made %d replacement(s).", len(matches)),
				},
			},
		}
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: fmt.Sprintf("File modified successfully. Made %d replacement(s) in %s (file size: %d bytes)",
					len(matches), originalPath, info.Size()),
			},
			mcp.EmbeddedResource{
				Type: "resource",
				Resource: mcp.TextResourceContents{
					URI:      resourceURI,
					MIMEType: "text/plain",
					Text:     fmt.Sprintf("Modified file: %s (%d bytes)", validPath, info.Size()),
				},
			},
		},
	}
}

// errorResult builds an error response.
func errorResult(msg string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: msg,
			},
		},
		IsError: true,
	}
}

// interpretEscapeSequences interprets common escape sequences in the replace string:
// \n -> newline, \r -> carriage return, \t -> tab, \\ -> backslash
func interpretEscapeSequences(s string) string {
	r := strings.NewReplacer(
		"\\n", "\n",
		"\\r", "\r",
		"\\t", "\t",
		"\\\\", "\\",
	)
	return r.Replace(s)
}

