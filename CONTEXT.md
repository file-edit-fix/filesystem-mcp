# CONTEXT.md

This file provides a glossary of domain terms used in the file-edit-fix project. It is read by engineering skills to ensure consistent vocabulary across issues, specs, and code.

## Project Context

`file-edit_fix` is a Go Workspace containing two submodules that solve Claude Code's Edit tool bugs on Windows (CRLF hash-check failures and Tab normalization issues).

## Terms

- **modify_file** — The core tool of this repository. A file content find-and-replace tool that serves as a fallback when Claude Code's built-in Edit tool fails on Windows. Operates at the filesystem level, bypassing the Edit tool chain entirely.
- **mixed matching** — The replacement strategy used by `modify_file`: try exact byte-level match first, fall back to line-level trim comparison (ignoring leading whitespace) when exact match fails.
- **dry_run** — A preview mode that reports what replacements would be made without actually writing to the file.
- **atomic write** — Write-to-temp-then-rename pattern that ensures the file is either fully updated or unchanged (no partial writes on failure).
- **escape sequence interpretation** — The `interpretEscapeSequences()` function that translates `\n`, `\r`, `\t`, `\\` in the replace string to actual characters before writing.
- **line-trim fallback** — When exact match fails, compares lines after stripping leading whitespace (`\t` and ` `). Allows matching code regardless of indentation differences.
- **byte-offset replacement** — Using `startByte`/`endByte` offsets rather than line-by-line reconstruction to apply replacements. Preserves unchanged content exactly.
- **CRLF normalization** — Converting `\r\n` to `\n` at read time so all matching operates on a consistent line-ending representation.
- **allowlist directory** — A directory configured at server startup that bounds all file access. Path validation ensures every resolved path falls within an allowed directory.
- **tool handler** — A function registered with the MCP server that processes a specific tool invocation. Each tool has one handler in `filesystemserver/handler/`.
