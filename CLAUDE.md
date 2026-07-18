# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
# Build
go build -o server .

# Test (all)
go test ./...

# Test (single package)
go test ./filesystemserver/...

# Test (single test, verbose)
go test -v -run TestReadfile_Valid ./filesystemserver/...

# Test with race detection
go test -race ./...

# Lint
go vet ./...

# Run
./server /path/to/allowed/dir
# Windows: AUTO expands to all available drives
./server AUTO
```

## Architecture

### Project: `github.com/bigmanBass666/filesystem-mcp`

A Go MCP (Model Context Protocol) server that provides secure filesystem access. Implements the MCP stdio transport protocol.

### Package Layout

- **`main.go`** — Entry point. Parses CLI args, supports `AUTO` keyword (Windows: expands to all available drives). Creates filesystem server and serves over stdio.
- **`filesystemserver/server.go`** — `NewFilesystemServer()` creates and registers all 14 tools + 1 resource (`file://`). Uses `github.com/mark3labs/mcp-go` for MCP server infrastructure.
- **`filesystemserver/handler.go`** — All tool handler implementations. Contains `FilesystemHandler` struct with `allowedDirs` for access control. Every handler validates paths via `validatePath()` before operating.

### Security Model

- **Allowed directories** — Server starts with a list of allowed directories. All paths are normalized to absolute form with trailing separators to prevent prefix matching attacks (e.g., `/tmp/foo` shouldn't match `/tmp/foobar`).
- **Path validation** (`validatePath`) — Converts to absolute → checks allowed directories → resolves symlinks → re-checks resolved path. For new files, validates parent directory.
- **Symlink safety** — `buildTree` and `validatePath` both resolve symlinks and verify the target stays within allowed directories.

### Tool Handlers (14 tools)

| Category | Tools |
|----------|-------|
| File I/O | `read_file`, `read_multiple_files`, `write_file`, `copy_file`, `move_file`, `delete_file`, `modify_file` |
| Directory | `list_directory`, `create_directory`, `tree` |
| Search | `search_files` (glob on names), `search_within_files` (substring in contents) |
| Info | `get_file_info`, `list_allowed_directories` |

Resource handler: `file://` URI scheme reads files/directories with MIME detection, size limits, and base64 encoding for binary files.

### Key Constants (`handler.go`)

- `MAX_INLINE_SIZE` = 5MB — files larger return a resource reference
- `MAX_BASE64_SIZE` = 1MB — binary files larger get a reference, not base64
- `MAX_SEARCHABLE_SIZE` = 10MB — files larger are skipped in `search_within_files`
- `MAX_SEARCH_RESULTS` = 1000 — cap on content search results

### MIME Detection

Uses `github.com/gabriel-vasile/mimetype` library. Three utility functions:
- `detectMimeType` — library detection with extension fallback
- `isTextFile` — text/plain + common application types (json, xml, yaml, etc.)
- `isImageFile` — image/* prefix

### Tree Building (`buildTree`)

Recursive directory traversal with configurable max depth. Returns `FileNode` JSON tree. Symlink handling controlled by `followSymlinks` parameter (default: false). Symlinks pointing outside allowed directories are skipped.

### Testing

- **In-package tests** (`handler_test.go`) — direct handler calls, test read/write/validate/search logic
- **External package tests** (`*_test.go` in `filesystemserver_test` package) — integration tests using in-process MCP client (`client.NewInProcessClient`)
- **Helper** (`utils_test.go`) — `startTestClient()` creates an initialized MCP client; `getTool()` retrieves a tool by name from the server
- Tests use `t.TempDir()` for temp directories and `testify` (`assert`/`require`)

### Docker

Multi-stage build (`golang:1.23-alpine` → `alpine:latest`). Default CMD passes `/app` as allowed directory.