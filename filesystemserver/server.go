package filesystemserver

import (
	"github.com/bigmanBass666/filesystem-mcp/filesystemserver/handler"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

var Version = "dev"

func NewFilesystemServer(allowedDirs []string) (*server.MCPServer, error) {

	h, err := handler.NewFilesystemHandler(allowedDirs)
	if err != nil {
		return nil, err
	}

	s := server.NewMCPServer(
		"secure-filesystem-server",
		Version,
	)

	s.AddTool(mcp.NewTool(
		"modify_file",
		mcp.WithDescription("Update file by finding and replacing text with intelligent matching. Uses exact match first, then line-level trim comparison as fallback for indentation differences. Supports batch operations via regex + all_occurrences. Use dry_run to preview matches before applying."),
		mcp.WithString("path",
			mcp.Description("Path to the file to modify (required when paths is not specified)"),
		),
		mcp.WithArray("paths",
			mcp.Description("Multiple file paths to modify with the same find/replace operation"),
		),
		mcp.WithString("find",
			mcp.Description("Text to search for (exact match or regex pattern when regex=true)"),
			mcp.Required(),
		),
		mcp.WithString("replace",
			mcp.Description("Text to replace with"),
			mcp.Required(),
		),
		mcp.WithBoolean("all_occurrences",
			mcp.Description("Replace all occurrences of the matching text (default: true)"),
		),
		mcp.WithBoolean("regex",
			mcp.Description("Treat the find pattern as a regular expression (default: false)"),
		),
		mcp.WithBoolean("dry_run",
			mcp.Description("Preview matches without modifying the file (default: false)"),
		),
	), h.HandleModifyFile)

	return s, nil
}
