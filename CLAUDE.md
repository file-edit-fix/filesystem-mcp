# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 项目上下文

本仓库是 `file_edit_fix` Go Workspace 的子模块（`go.work` 挂在 `D:\Work\Projects\file_edit_fix\`），与 `claude-tab-fix` 共同解决 Claude Code 在 Windows 上的 Edit 工具 bug。**`modify_file` 工具是本项目的核心功能**——作为 Edit 工具失败（CRLF 哈希检查 #13456/#54876、Tab 显示归一化 #26996）时的后备方案，直接在文件系统层面做查找替换，不经过 Edit 工具链。

## Commands

```bash
# Build
go build -o server .

# Install to ~/go/bin (for MCP config)
go install .

# Test (all)
go test ./...

# Test (single package)
go test ./filesystemserver/...

# Test (single test, verbose)
go test -v -run TestModifyFile_BasicReplace ./filesystemserver/handler/

# Test with race detection
go test -race ./...

# Lint
go vet ./...

# Dependencies
go mod tidy

# Run (specify allowed directories)
./server /path/to/allowed/dir
# Windows: AUTO expands to all available drives (C:\, D:\, etc.)
./server AUTO
```

## Architecture

### Project: `github.com/bigmanBass666/filesystem-mcp`

A Go MCP (Model Context Protocol) server that provides secure filesystem access. Implements the MCP stdio transport protocol. **Fork of [mark3labs/mcp-filesystem-server](https://github.com/mark3labs/mcp-filesystem-server)** — the primary addition is `modify_file` for Claude Code Windows Edit tool fallback.

### Package Layout

- **`main.go`** — Entry point. Parses CLI args, supports `AUTO` keyword (Windows: expands to all available drives). Creates filesystem server and serves over stdio.
- **`filesystemserver/server.go`** — `NewFilesystemServer()` creates and registers all 14 tools + 1 resource (`file://`). Uses `github.com/mark3labs/mcp-go` for MCP server infrastructure. Exports `Version` variable (build-time injected via `-ldflags -X`, defaults to `"dev"`).
- **`filesystemserver/handler/handler.go`** — `FilesystemHandler` struct with `allowedDirs` for access control. Constructor normalizes allowed directories (trailing separator to prevent prefix matching attacks).
- **`filesystemserver/handler/helper.go`** — Path validation (`validatePath`), MIME detection, `buildTree`, `resolvePath` (`.` / `./` handling).
- **`filesystemserver/handler/`** — One file per tool or related group:
  `read_file.go`, `write_file.go`, `delete_file.go`, `copy_file.go`, `move_file.go`, `modify_file.go` (file I/O),
  `list_directory.go`, `create_directory.go`, `tree.go` (directory operations),
  `search_files.go`, `search_within_files.go` (search),
  `get_file_info.go`, `list_allowed_directories.go` (info),
  `read_multiple_files.go` (batch read),
  `resources.go` (`file://` resource handler),
  `types.go` (shared types: `FileInfo`, `FileNode`, `SearchResult`, constants).

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

### Key Constants (`types.go`)

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

### `modify_file` Tool

This fork's key addition over the upstream. A find-and-replace tool for file content, serving as the fallback when Claude Code's built-in Edit tool fails on Windows (CRLF hash check, tab normalization).

**Parameters:** `path` (required), `find` (required), `replace` (required), `all_occurrences` (default: true), `regex` (default: false).

**Behavior:**
- Exact-match mode uses `strings.ReplaceAll` / `strings.Index` — **byte-level exact match**, no interpretation of escape sequences
- Regex mode uses Go `regexp` — `$1` capture group references work, but `\n` / `\t` in `replace` are literal characters, not escape sequences
- 0-replacement (no match) returns success silently — the caller must check `"Made 0 replacement(s)"` in the response text

### File Info

`get_file_info` uses `github.com/djherbis/times` to retrieve file creation/access/modification timestamps. Not all platforms support all three timestamps.

### Testing

- **In-package tests** (`handler/*_test.go`) — one test file per handler, using `NewFilesystemHandler` + `resolveAllowedDirs` helper. `modify_file_test.go` covers 8 scenarios: basic replace, single replace, regex replace, file not found, invalid regex, directory rejection, access denied, no match.
- **External package tests** (`filesystemserver/*_test.go` in `filesystemserver_test` package) — integration tests using in-process MCP client (`client.NewInProcessClient`). Includes `server_test.go` (schema regression test for `read_multiple_files`).
- **Helper** (`utils_test.go`) — `startTestClient()` creates an initialized MCP client; `getTool()` retrieves a tool by name from the server.
- Tests use `t.TempDir()` for temp directories and `testify` (`assert`/`require`).

### CI/CD

- **Test** (`.github/workflows/test.yml`) — runs on every push, matrix across ubuntu/windows/macos, `go test -race ./...`
- **Release** (`.github/workflows/release.yml`) — triggered by `v*` tags, runs GoReleaser for multi-platform binaries + Docker build/push to `ghcr.io` (linux/amd64 + linux/arm64)
- **Dependabot** (`.github/dependabot.yml`) — weekly Go dependency updates

### GoReleaser (`.goreleaser.yml`)

Cross-platform builds: linux/windows/darwin × amd64/arm64. Version injection via `-ldflags -X github.com/bigmanBass666/filesystem-mcp/filesystemserver.Version={{.Version}}`. Publishes to GitHub Releases (`bigmanBass666/mcp-filesystem-server`). Archives include README.md and LICENSE.

### Deployment

- **Docker** — Multi-stage build (`golang:1.23-alpine` → `alpine:latest`). Default CMD passes `/app` as allowed directory.
- **Smithery** — `smithery.yaml` provides stdio start command configuration for Smithery.ai deployment, with `allowedDirectory` and `additionalDirectories` params.