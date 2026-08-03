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

	if normalizedFind == "" {
		return nil, nil
	}

	// Try exact match first
	var matches []matchResult
	offset := 0
	for {
		idx := strings.Index(content[offset:], normalizedFind)
		if idx < 0 {
			break
		}
		startByte := offset + idx
		endByte := startByte + len(normalizedFind)
		matches = append(matches, matchResult{
			startByte:   startByte,
			endByte:     endByte,
			replacement: normalizedReplace,
		})
		if !allOccurrences {
			break
		}
		offset = endByte
	}
	if len(matches) > 0 {
		return matches, nil
	}

	// Exact match failed — try line-level trim fallback
	findLines := strings.Split(normalizedFind, "\n")
	contentLines := strings.Split(content, "\n")
	contentOffsets := lineOffsets(content)

	if len(findLines) > len(contentLines) {
		return nil, nil
	}

	for i := 0; i <= len(contentLines)-len(findLines); i++ {
		window := contentLines[i : i+len(findLines)]
		if linesTrimMatch(findLines, window) {
			return []matchResult{{
				startByte:   contentOffsets[i],
				endByte:     contentOffsets[i+len(findLines)],
				replacement: normalizedReplace,
			}}, nil
		}
	}

	return nil, nil
}

// linesTrimMatch returns true if every line in findLines matches the
// corresponding line in contentLines when both are trimmed of leading whitespace.
// Empty lines in findLines must match empty lines in contentLines exactly
// (a whitespace-only line is not considered empty).
func linesTrimMatch(findLines, contentLines []string) bool {
	if len(findLines) > len(contentLines) {
		return false
	}
	for i, fl := range findLines {
		cl := contentLines[i]
		flTrimmed := strings.TrimLeft(fl, "\t ")
		if flTrimmed == "" {
			if cl != "" {
				return false
			}
			continue
		}
		if flTrimmed != strings.TrimLeft(cl, "\t ") {
			return false
		}
	}
	return true
}

// regexReplace performs replacement using regex patterns.
func regexReplace(content, find, replace string, allOccurrences bool) ([]matchResult, error) {
	normalizedFind := strings.ReplaceAll(find, "\r\n", "\n")

	if normalizedFind == "" {
		return nil, nil
	}

	re, err := regexp.Compile(normalizedFind)
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
// When matches overlap, only the first match's replacement is applied and
// the overlapping region of subsequent matches is skipped.
func applyReplacements(content string, matches []matchResult) string {
	if len(matches) == 0 {
		return content
	}

	var result strings.Builder
	writtenUpTo := matches[0].startByte
	result.WriteString(content[:writtenUpTo])

	for i := 0; i < len(matches); i++ {
		result.WriteString(matches[i].replacement)
		if i+1 < len(matches) {
			writtenUpTo = matches[i].endByte
			nextStart := matches[i+1].startByte
			if writtenUpTo < nextStart {
				result.WriteString(content[writtenUpTo:nextStart])
			}
		}
	}
	result.WriteString(content[matches[len(matches)-1].endByte:])

	return normalizeBlankLines(result.String())
}

// normalizeBlankLines collapses runs of 3+ consecutive blank lines into
// exactly 2 blank lines. This prevents gaps like triple-empty-lines after
// deleting import lines or other block-structured content.
func normalizeBlankLines(s string) string {
	re := regexp.MustCompile(`(\n){4,}`)
	return re.ReplaceAllString(s, "$1$1")
}

// lineOffsets returns a slice where offsets[i] is the byte offset of line i in text.
func lineOffsets(text string) []int {
	lines := strings.Split(text, "\n")
	offsets := make([]int, len(lines))
	offset := 0
	for i, line := range lines {
		offsets[i] = offset
		if i < len(lines)-1 {
			offset += len(line) + 1
		} else {
			offset += len(line)
		}
	}
	return offsets
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
func successResult(requestedPath, validatedPath string, matches []matchResult) *mcp.CallToolResult {
	resourceURI := pathToResourceURI(validatedPath)

	info, err := os.Stat(validatedPath)
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
					len(matches), requestedPath, info.Size()),
			},
			mcp.EmbeddedResource{
				Type: "resource",
				Resource: mcp.TextResourceContents{
					URI:      resourceURI,
					MIMEType: "text/plain",
					Text:     fmt.Sprintf("Modified file: %s (%d bytes)", validatedPath, info.Size()),
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

