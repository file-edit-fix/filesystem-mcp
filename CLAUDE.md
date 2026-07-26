# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 项目上下文

本仓库是 `file_edit_fix` Go Workspace 的子模块（`go.work` 挂在 `D:\Work\Projects\file_edit_fix\`），与 `claude-tab-fix` 共同解决 Claude Code 在 Windows 上的 Edit 工具 bug。**`modify_file` 工具是本项目的核心功能**——作为 Edit 工具失败（CRLF 哈希检查 #13456/#54876、Tab 显示归一化 #26996）时的后备方案，直接在文件系统层面做查找替换，不经过 Edit 工具链。

## 命令

```bash
# 构建
go build -o server .

# 安装到 ~/go/bin（MCP 配置用这个）
go install .

# 全部测试
go test ./...

# 单个包测试
go test ./filesystemserver/...

# 单个测试（详细输出）
go test -v -run TestModifyFile_BasicReplace ./filesystemserver/handler/

# 竞争检测测试
go test -race ./...

# 静态分析
go vet ./...

# 依赖管理
go mod tidy

# 运行（指定允许目录）
./server /path/to/allowed/dir
# Windows: AUTO 展开为所有可用驱动器（C:\, D:\ 等）
./server AUTO
```

## 架构

### 项目：`github.com/bigmanBass666/filesystem-mcp`

一个 Go 实现的 MCP 文件系统访问服务器，使用 MCP stdio 传输协议。**Fork 自 [mark3labs/mcp-filesystem-server](https://github.com/mark3labs/mcp-filesystem-server)** — 主要新增了 `modify_file` 工具，作为 Claude Code Windows Edit 工具的后备方案。

### 包布局

- **`main.go`** — 入口点。解析 CLI 参数，支持 `AUTO` 关键字（Windows 上展开为所有可用驱动器）。创建服务器并通过 stdio 提供服务。
- **`filesystemserver/server.go`** — `NewFilesystemServer()` 创建并注册所有 14 个工具 + 1 个资源处理器（`file://`）。使用 `github.com/mark3labs/mcp-go` 作为 MCP 服务器基础设施。导出 `Version` 变量（构建时通过 `-ldflags -X` 注入，默认 `"dev"`）。
- **`filesystemserver/handler/handler.go`** — `FilesystemHandler` 结构体，持有 `allowedDirs` 用于访问控制。构造函数归一化允许目录（追加分隔符防止前缀匹配攻击）。
- **`filesystemserver/handler/helper.go`** — 路径验证（`validatePath`）、MIME 检测、`buildTree`、`resolvePath`（`.` / `./` 相对路径处理）。
- **`filesystemserver/handler/`** — 每个工具一个文件或一组相关功能：
  `read_file.go`, `write_file.go`, `delete_file.go`, `copy_file.go`, `move_file.go`, `modify_file.go`（文件 I/O），
  `list_directory.go`, `create_directory.go`, `tree.go`（目录操作），
  `search_files.go`, `search_within_files.go`（搜索），
  `get_file_info.go`, `list_allowed_directories.go`（信息），
  `read_multiple_files.go`（批量读取），
  `resources.go`（`file://` 资源处理器），
  `types.go`（共享类型：`FileInfo`、`FileNode`、`SearchResult`、常量）。

### 安全模型

- **允许目录** — 服务器启动时指定允许的目录列表。所有路径归一化为绝对路径，末尾追加分隔符，防止前缀匹配攻击（如 `/tmp/foo` 不应匹配 `/tmp/foobar`）。
- **路径验证**（`validatePath`）— 转绝对路径 → 检查允许目录 → 解析符号链接 → 重新检查解析后的路径。对于新文件，验证父目录。
- **符号链接安全** — `buildTree` 和 `validatePath` 均解析符号链接并验证目标在允许目录内。

### 工具处理器（14 个工具）

| 分类 | 工具 |
|----------|-------|
| 文件 I/O | `read_file`, `read_multiple_files`, `write_file`, `copy_file`, `move_file`, `delete_file`, `modify_file` |
| 目录操作 | `list_directory`, `create_directory`, `tree` |
| 搜索 | `search_files`（按文件名 glob 匹配）、`search_within_files`（按文件内容子串搜索） |
| 信息 | `get_file_info`, `list_allowed_directories` |

资源处理器：`file://` URI 方案，支持 MIME 类型检测、大小限制、二进制文件 base64 编码。

### 关键常量（`types.go`）

- `MAX_INLINE_SIZE` = 5MB — 超过此大小的文件返回资源引用，不内联
- `MAX_BASE64_SIZE` = 1MB — 超过此大小的二进制文件返回引用，不做 base64
- `MAX_SEARCHABLE_SIZE` = 10MB — `search_within_files` 跳过超过此大小的文件
- `MAX_SEARCH_RESULTS` = 1000 — 内容搜索结果上限

### MIME 检测

使用 `github.com/gabriel-vasile/mimetype` 库。三个工具函数：
- `detectMimeType` — 库检测 + 扩展名回退
- `isTextFile` — 判断是否为文本文件（text/* + 常见 application 类型如 json, xml, yaml 等）
- `isImageFile` — 判断是否为图片（image/* 前缀）

### 目录树构建（`buildTree`）

递归目录遍历，可配置最大深度。返回 `FileNode` JSON 树。`followSymlinks` 参数控制是否跟随符号链接（默认 false）。指向允许目录外的符号链接会被跳过。

### `modify_file` 工具

本 fork 相对于上游的关键新增。一个文件内容查找替换工具，作为 Claude Code 内置 Edit 工具在 Windows 上失败（CRLF 哈希检查、Tab 归一化）时的后备方案。

**参数：** `path`（必填）、`find`（必填）、`replace`（必填）、`all_occurrences`（默认 true）、`regex`（默认 false）。

**行为说明：**
- 精确匹配模式使用 `strings.ReplaceAll` / `strings.Index` — **字节级精确匹配**，不解释转义序列
- 正则模式使用 Go `regexp` — `$1` 捕获组引用正常生效，但 `replace` 中的 `\n` / `\t` 是字面字符，不是转义序列
- 0 次匹配（未找到）静默返回成功 — 调用方需检查响应中的 `"Made 0 replacement(s)"` 来判断

### 文件信息

`get_file_info` 使用 `github.com/djherbis/times` 获取文件的创建/访问/修改时间戳。并非所有平台都支持全部三个时间戳。

### 测试

- **包内测试**（`handler/*_test.go`）— 每个处理器一个测试文件，使用 `NewFilesystemHandler` + `resolveAllowedDirs` 辅助函数。`modify_file_test.go` 覆盖 8 种场景：基本替换、单次替换、正则替换、文件不存在、无效正则、目录拒绝、越界拒绝、无匹配。
- **外部包测试**（`filesystemserver/*_test.go`，在 `filesystemserver_test` 包中）— 集成测试，使用进程内 MCP 客户端（`client.NewInProcessClient`）。包括 `server_test.go`（`read_multiple_files` 的 schema 回归测试）。
- **辅助函数**（`utils_test.go`）— `startTestClient()` 创建并初始化 MCP 客户端；`getTool()` 按名称从服务器获取工具定义。
- 测试使用 `t.TempDir()` 创建临时目录，`testify`（`assert`/`require`）进行断言。

### CI/CD

- **测试**（`.github/workflows/test.yml`）— 每次 push 触发，在 ubuntu/windows/macos 三平台运行 `go test -race ./...`
- **发布**（`.github/workflows/release.yml`）— 推送 `v*` 标签触发，运行 GoReleaser 构建多平台二进制 + Docker 构建并推送到 `ghcr.io`（linux/amd64 + linux/arm64）
- **依赖更新**（`.github/dependabot.yml`）— 每周检查 Go 依赖更新

### GoReleaser（`.goreleaser.yml`）

跨平台构建：linux/windows/darwin × amd64/arm64。版本注入方式：`-ldflags -X github.com/bigmanBass666/filesystem-mcp/filesystemserver.Version={{.Version}}`。发布到 GitHub Releases（`bigmanBass666/mcp-filesystem-server`）。归档包含 README.md 和 LICENSE。

### 部署

- **Docker** — 多阶段构建（`golang:1.23-alpine` → `alpine:latest`）。默认 CMD 传入 `/app` 作为允许目录。
- **Smithery** — `smithery.yaml` 提供 Smithery.ai 部署的 stdio 启动命令配置，支持 `allowedDirectory` 和 `additionalDirectories` 参数。