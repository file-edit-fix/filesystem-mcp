package handler

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// handleModifyFile handles the modify_file tool request
func (fs *FilesystemHandler) HandleModifyFile(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	// Extract arguments
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

	// Extract optional arguments with defaults
	allOccurrences := true // Default value
	if val, err := request.RequireBool("all_occurrences"); err == nil {
		allOccurrences = val
	}

	useRegex := false // Default value
	if val, err := request.RequireBool("regex"); err == nil {
		useRegex = val
	}

	// Handle empty or relative paths like "." or "./" by converting to absolute path
	resolvedPath, err := resolvePath(path)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Error: %v", err),
				},
			},
			IsError: true,
		}, nil
	}
	path = resolvedPath

	// Validate path is within allowed directories
	validPath, err := fs.validatePath(path)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Error: %v", err),
				},
			},
			IsError: true,
		}, nil
	}

	// Check if it's a directory
	if info, err := os.Stat(validPath); err == nil && info.IsDir() {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: "Error: Cannot modify a directory",
				},
			},
			IsError: true,
		}, nil
	}

	// Check if file exists
	if _, err := os.Stat(validPath); os.IsNotExist(err) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Error: File not found: %s", path),
				},
			},
			IsError: true,
		}, nil
	}

	// Read file content
	content, err := os.ReadFile(validPath)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Error reading file: %v", err),
				},
			},
			IsError: true,
		}, nil
	}

	originalContent := string(content)
	modifiedContent := ""
	replacementCount := 0

	// Perform the replacement
	if useRegex {
		re, err := regexp.Compile(find)
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.TextContent{
						Type: "text",
						Text: fmt.Sprintf("Error: Invalid regular expression: %v", err),
					},
				},
				IsError: true,
			}, nil
		}

		if allOccurrences {
			modifiedContent = re.ReplaceAllString(originalContent, interpretEscapeSequences(replace))
			replacementCount = len(re.FindAllString(originalContent, -1))
		} else {
			matched := re.FindStringIndex(originalContent)
			if matched != nil {
				replacementCount = 1
				modifiedContent = originalContent[:matched[0]] + interpretEscapeSequences(replace) + originalContent[matched[1]:]
			} else {
				modifiedContent = originalContent
				replacementCount = 0
				// Fuzzy fallback: try context-aware matching when exact match fails
				if replacementCount == 0 {
					fuzzyContent, fuzzyCount := contextAwareReplace(originalContent, find, replace)
					if fuzzyCount > 0 {
						modifiedContent = fuzzyContent
						replacementCount = fuzzyCount
					}
				}
			}
		}
	} else {
		// Normalize CRLF to LF for reliable matching
		normalizedContent := strings.ReplaceAll(originalContent, "\r\n", "\n")
		normalizedFind := strings.ReplaceAll(find, "\r\n", "\n")
		normalizedReplace := interpretEscapeSequences(replace)

		if allOccurrences {
			replacementCount = strings.Count(normalizedContent, normalizedFind)
			modifiedContent = strings.ReplaceAll(normalizedContent, normalizedFind, normalizedReplace)
		} else {
			if index := strings.Index(normalizedContent, normalizedFind); index != -1 {
				replacementCount = 1
				modifiedContent = normalizedContent[:index] + normalizedReplace + normalizedContent[index+len(normalizedFind):]
			} else {
				modifiedContent = normalizedContent
				replacementCount = 0
				// Fuzzy fallback: try context-aware matching on normalized content
				if replacementCount == 0 {
					fuzzyContent, fuzzyCount := contextAwareReplace(normalizedContent, normalizedFind, normalizedReplace)
					if fuzzyCount > 0 {
						modifiedContent = fuzzyContent
						replacementCount = fuzzyCount
					}
				}
			}
		}
	}

	// Write modified content back to file
	if err := os.WriteFile(validPath, []byte(modifiedContent), 0644); err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Error writing to file: %v", err),
				},
			},
			IsError: true,
		}, nil
	}

	// Create response
	resourceURI := pathToResourceURI(validPath)

	// Get file info for the response
	info, err := os.Stat(validPath)
	if err != nil {
		// File was written but we couldn't get info
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("File modified successfully. Made %d replacement(s).", replacementCount),
				},
			},
		}, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: fmt.Sprintf("File modified successfully. Made %d replacement(s) in %s (file size: %d bytes)",
					replacementCount, path, info.Size()),
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
	}, nil
}

// interpretEscapeSequences interprets common escape sequences in the replace string:
// \n → newline, \r → carriage return, \t → tab, \\ → backslash
// This is needed because Go's regexp.ReplaceAllString and strings.ReplaceAll
// treat \t and \n as literal characters.
func interpretEscapeSequences(s string) string {
	r := strings.NewReplacer(
		"\\n", "\n",
		"\\r", "\r",
		"\\t", "\t",
		"\\\\", "\\",
	)
	return r.Replace(s)
}

// lineSimilarity returns the ratio of common words between two lines.
// Leading indentation is normalized before comparison so tab/space differences
// don't penalize the score.
func lineSimilarity(a, b string) float64 {
	// Strip leading indentation (tabs and spaces) for comparison
	trimmedA := strings.TrimLeft(a, "\t ")
	trimmedB := strings.TrimLeft(b, "\t ")
	wa := strings.Fields(trimmedA)
	wb := strings.Fields(trimmedB)
	if len(wa) == 0 && len(wb) == 0 {
		return 1.0
	}
	if len(wa) == 0 || len(wb) == 0 {
		return 0.0
	}

	ws := make(map[string]bool)
	for _, w := range wb {
		ws[strings.ToLower(w)] = true
	}
	common := 0
	for _, w := range wa {
		if ws[strings.ToLower(w)] {
			common++
		}
	}
	return float64(common*2) / float64(len(wa)+len(wb))
}

// fuzzyFindBlock searches for the best matching position of findBlock
// within fileLines using a sliding window with LCS line similarity.
// It returns the byte offset and the matched block text, or (-1, "") if no good match.
func fuzzyFindBlock(fileLines, findLines []string, threshold float64) (int, string) {
	if len(findLines) == 0 || len(fileLines) < len(findLines) {
		return -1, ""
	}

	bestScore := 0.0
	bestOffset := -1

	for i := 0; i <= len(fileLines)-len(findLines); i++ {
		window := fileLines[i : i+len(findLines)]
		totalSim := 0.0
		for j, fl := range findLines {
			totalSim += lineSimilarity(fl, window[j])
		}
		avgSim := totalSim / float64(len(findLines))

		if avgSim >= threshold && avgSim > bestScore {
			bestScore = avgSim
			bestOffset = i
		}
	}

	if bestOffset == -1 {
		return -1, ""
	}
	matched := strings.Join(fileLines[bestOffset:bestOffset+len(findLines)], "\n")
	return bestOffset, matched
}

// lineOffset converts a line index and column within that line to a byte offset in the full text.
func lineOffset(text string, lineIndex int) int {
	lines := strings.Split(text, "\n")
	offset := 0
	for i := 0; i < lineIndex && i < len(lines); i++ {
		offset += len(lines[i]) + 1 // +1 for \n
	}
	return offset
}

// contextAwareReplace attempts a fuzzy match fallback when exact and regex matching fail.
// It splits find and file content into lines, extracts surrounding context from the find block,
// and searches for a best-match position using line-level similarity.
func contextAwareReplace(content, find, replace string) (string, int) {
	findLines := strings.Split(find, "\n")
	if len(findLines) == 0 {
		return content, 0
	}

	// Extract context lines: first 2 and last 2 non-empty lines of the find block
	var contextLines []string
	for _, l := range findLines {
		if strings.TrimSpace(l) != "" {
			contextLines = append(contextLines, l)
		}
	}
	if len(contextLines) > 4 {
		contextLines = append(contextLines[:2], contextLines[len(contextLines)-2:]...)
	}

	fileLines := strings.Split(content, "\n")

	// Try with full find block first (threshold 0.80)
	if offset, _ := fuzzyFindBlock(fileLines, findLines, 0.80); offset != -1 {
		start := lineOffset(content, offset)
		end := lineOffset(content, offset+len(findLines))
		newContent := content[:start] + replace + content[end:]
		return newContent, 1
	}

	// Try with context lines only (lower threshold 0.70)
	if len(contextLines) > 0 {
		if offset, _ := fuzzyFindBlock(fileLines, contextLines, 0.70); offset != -1 {
			// Replace within the matched region — expand to include lines between context anchors
			startLine := offset
			endLine := offset + len(contextLines)

			// Expand to cover full find block height if context is a subset
			if len(contextLines) < len(findLines) {
				expansion := (len(findLines) - len(contextLines)) / 2
				startLine = max(0, offset-expansion)
				endLine = min(len(fileLines), offset+len(contextLines)+expansion)
			}

			start := lineOffset(content, startLine)
			end := lineOffset(content, endLine)
			
			newContent := content[:start] + replace + content[end:]
			 // matched text logged in verbose mode if needed
			return newContent, 1
		}
	}

	return content, 0
}