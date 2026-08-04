package handler

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// matchResult holds the outcome of a matching attempt using byte offsets.
type matchResult struct {
	startByte   int    // byte offset where match starts in content
	endByte     int    // byte offset where match ends (exclusive)
	replacement string // the replacement text to inject
}

var reMadeReplacements = regexp.MustCompile(`Made (\d+) replacement`)
var reDryRunMatches    = regexp.MustCompile(`Dry run: (\d+) match`)

// HandleModifyFile handles the modify_file tool request
func (fs *FilesystemHandler) HandleModifyFile(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
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

	argsMap, ok := request.Params.Arguments.(map[string]any)
	if !ok {
		argsMap = map[string]any{}
	}

	paths, hasPaths := argsMap["paths"]
	if hasPaths {
		pathSlice, err := toStringSlice(paths)
		if err != nil {
			return errorResult(err.Error()), nil
		}
		if len(pathSlice) == 0 {
			return errorResult("Error: paths array must not be empty"), nil
		}
		return fs.batchModifyResult(ctx, pathSlice, find, replace, useRegex, allOccurrences, dryRun)
	}

	if _, hasPath := argsMap["path"]; !hasPath {
		return errorResult("Error: either path or paths must be specified"), nil
	}

	path, err := request.RequireString("path")
	if err != nil {
		return nil, err
	}

	return fs.modifyFileSingle(ctx, path, find, replace, useRegex, allOccurrences, dryRun)
}

// batchModifyResult processes the same find/replace across multiple files,
// building an aggregate summary of all results.
func (fs *FilesystemHandler) batchModifyResult(
	ctx context.Context,
	paths []string,
	find, replace string,
	useRegex, allOccurrences, dryRun bool,
) (*mcp.CallToolResult, error) {
	totalMatches := 0
	modifiedCount := 0

	var sb strings.Builder

	if dryRun {
		sb.WriteString(fmt.Sprintf("Batch dry run: %d file(s)\n\n", len(paths)))
	} else {
		sb.WriteString(fmt.Sprintf("Batch modify completed: 0/%d files modified\n\n", len(paths)))
	}

	for _, p := range paths {
		result, err := fs.modifyFileSingle(ctx, p, find, replace, useRegex, allOccurrences, dryRun)
		if err != nil {
			sb.WriteString(fmt.Sprintf("  %s: Error — %v\n", p, err))
			continue
		}

		if result.IsError {
			text := result.Content[0].(mcp.TextContent).Text
			sb.WriteString(fmt.Sprintf("  %s: Error — %s\n", p, text))
			continue
		}

		// Parse the success/no-match text to extract replacement count
		text := result.Content[0].(mcp.TextContent).Text
		count := parseReplacementCount(text)
		if count > 0 {
			totalMatches += count
			modifiedCount++
			if dryRun {
				_, contentStr, matches, _, _ := fs.findMatches(ctx, p, find, replace, useRegex, allOccurrences)
				sb.WriteString(fmt.Sprintf("  %s:\n", p))
				sb.WriteString(formatDryRunMatches(contentStr, matches, "    "))
			} else {
				sb.WriteString(fmt.Sprintf("  %s: %d replacement(s)\n", p, count))
			}
		} else {
			// No matches
			sb.WriteString(fmt.Sprintf("  %s: No matches\n", p))
		}
	}

	// Update the header with final counts
	header := sb.String()
	if dryRun {
		header = strings.Replace(header,
			fmt.Sprintf("Batch dry run: %d file(s)", len(paths)),
			fmt.Sprintf("Batch dry run: %d file(s), %d match(es) found", len(paths), totalMatches),
			1)
	} else {
		header = strings.Replace(header,
			fmt.Sprintf("Batch modify completed: 0/%d files modified", len(paths)),
			fmt.Sprintf("Batch modify completed: %d/%d files modified", modifiedCount, len(paths)),
			1)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: header,
			},
		},
	}, nil
}

// parseReplacementCount extracts the number of replacements from a
// modifyFileSingle success/no-match/dry-run result text.
func parseReplacementCount(text string) int {
	matches := reMadeReplacements.FindStringSubmatch(text)
	if len(matches) > 1 {
		count, _ := strconv.Atoi(matches[1])
		return count
	}
	matches = reDryRunMatches.FindStringSubmatch(text)
	if len(matches) > 1 {
		count, _ := strconv.Atoi(matches[1])
		return count
	}
	return 0
}

// findMatches reads a file, validates it, and returns the resolved path,
// content with CRLF line endings normalized to LF, and computed matches.
func (fs *FilesystemHandler) findMatches(
	ctx context.Context,
	path, find, replace string,
	useRegex, allOccurrences bool,
) (resolvedPath, contentStr string, matches []matchResult, result *mcp.CallToolResult, err error) {
	var err2 error
	resolvedPath, err2 = resolvePath(path)
	if err2 != nil {
		return "", "", nil, errorResult(fmt.Sprintf("Error: %v", err2)), nil
	}
	path = resolvedPath

	validPath, err2 := fs.validatePath(path)
	if err2 != nil {
		return "", "", nil, errorResult(fmt.Sprintf("Error: %v", err2)), nil
	}

	if info, err2 := os.Stat(validPath); err2 == nil && info.IsDir() {
		return "", "", nil, errorResult("Error: Cannot modify a directory"), nil
	}

	if _, err2 := os.Stat(validPath); os.IsNotExist(err2) {
		return "", "", nil, errorResult(fmt.Sprintf("Error: File not found: %s", path)), nil
	}

	content, err2 := os.ReadFile(validPath)
	if err2 != nil {
		return "", "", nil, errorResult(fmt.Sprintf("Error reading file: %v", err2)), nil
	}

	contentStr = strings.ReplaceAll(string(content), "\r\n", "\n")

	if useRegex {
		matches, err2 = regexReplace(contentStr, find, replace, allOccurrences)
	} else {
		matches, err2 = mixedReplace(contentStr, find, replace, allOccurrences)
	}

	if err2 != nil {
		return "", "", nil, errorResult(fmt.Sprintf("Error: Invalid regular expression: %v", err2)), nil
	}

	return resolvedPath, contentStr, matches, nil, nil
}

// modifyFileSingle processes a single file modification request.
func (fs *FilesystemHandler) modifyFileSingle(
	ctx context.Context,
	path, find, replace string,
	useRegex, allOccurrences, dryRun bool,
) (*mcp.CallToolResult, error) {
	resolvedPath, contentStr, matches, result, err := fs.findMatches(ctx, path, find, replace, useRegex, allOccurrences)
	if err != nil || result != nil {
		return result, err
	}

	if len(matches) == 0 {
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
		return dryRunResult(contentStr, matches), nil
	}

	modifiedContent := applyReplacements(contentStr, matches)
	if err := atomicWriteFile(resolvedPath, modifiedContent); err != nil {
		return errorResult(fmt.Sprintf("Error writing to file: %v", err)), nil
	}

	return successResult(path, resolvedPath, matches), nil
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

// applyReplacements builds the output by weaving replacements into the
// original content. Non-overlapping gaps between consecutive matches are
// copied verbatim; overlapping region of later matches is silently dropped.
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

// formatDryRunMatches returns the per-match detail lines for a dry run result,
// without any header. Each line is prefixed with the given indent string.
func formatDryRunMatches(originalContent string, matches []matchResult, indent string) string {
	var sb strings.Builder
	for i, m := range matches {
		startLine := strings.Count(originalContent[:m.startByte], "\n")
		sb.WriteString(fmt.Sprintf("%sMatch %d at line %d:\n", indent, i+1, startLine+1))
		for _, line := range strings.Split(originalContent[m.startByte:m.endByte], "\n") {
			sb.WriteString(fmt.Sprintf("%s  %s\n", indent, line))
		}
		sb.WriteString(fmt.Sprintf("%s  -> would be replaced with:\n", indent))
		for _, rl := range strings.Split(m.replacement, "\n") {
			sb.WriteString(fmt.Sprintf("%s    %s\n", indent, rl))
		}
		sb.WriteString(fmt.Sprintf("%s\n", indent))
	}
	return sb.String()
}

// dryRunResult builds a dry_run response listing matched lines.
func dryRunResult(originalContent string, matches []matchResult) *mcp.CallToolResult {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Dry run: %d match(es) found (no changes applied)\n\n", len(matches)))
	sb.WriteString(formatDryRunMatches(originalContent, matches, ""))

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

// toStringSlice converts []interface{} or []string to []string.
func toStringSlice(v interface{}) ([]string, error) {
	if ss, ok := v.([]string); ok {
		return ss, nil
	}
	if raw, ok := v.([]interface{}); ok {
		result := make([]string, len(raw))
		for i, item := range raw {
			s, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("Error: paths must be an array of strings")
			}
			result[i] = s
		}
		return result, nil
	}
	return nil, fmt.Errorf("Error: paths must be an array")
}

