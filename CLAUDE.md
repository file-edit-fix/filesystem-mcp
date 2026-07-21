# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 命令

```bash
# 构建
go build -o server .

# 全部测试
go test ./...

# 单个包测试
go test ./filesystemserver/...
go test ./filesystemserver/handler/...

# 竞争检测测试
go test -race ./...

# 依赖管理
go mod tidy

# 静态分析
go vet ./...

# 运行
./server /path/to/allowed/dir
# Windows: AUTO 自动展开所有可用驱动器
./server AUTO
```

## 项目总览

**模块路径：** `github.com/bigmanBass666/filesystem-mcp`（Go 1.23.2）
**MCP 框架：** `github.com/mark3labs/mcp-go v0.32.0`
**协议：** MCP stdio 传输

该项目是一个 Go 实现的 MCP (Model Context Protocol) 文件系统访问服务器，提供 14 个工具和 1 个 `file://` 资源处理器。

## 包布局

- **`main.go`** — 入口点。解析 CLI 参数，支持 `AUTO` 关键字（Windows 上展开为所有可用驱动器）。创建服务器并通过 stdio 提供服务。
- **`filesystemserver/server.go`** — `NewFilesystemServer()` 创建并注册所有 14 个工具和 1 个资源处理器。`Version` 变量（默认 `"dev"`）可通过 `-ldflags -X` 在构建时注入。
- **`filesystemserver/handler/`** — 所有处理器实现目录，每个文件对应一个工具或一组相关功能：
  - `handler.go` — `FilesystemHandler` 结构体，持有 `allowedDirs`
  - `helper.go` — 路径验证、MIME 检测、树构建等辅助函数
  - `read_file.go`, `write_file.go`, `delete_file.go`, `copy_file.go`, `move_file.go`, `modify_file.go` — 文件 I/O 处理器
  - `list_directory.go`, `create_directory.go`, `tree.go` — 目录操作处理器
  - `search_files.go`, `search_within_files.go` — 搜索处理器
  - `get_file_info.go`, `list_allowed_directories.go` — 信息处理器
  - `read_multiple_files.go` — 批量读取处理器
  - `resources.go` — `file://` 资源处理器
  - `types.go` — 共享类型定义

## 安全模型

- **允许目录** — 服务器启动时指定允许的目录列表。所有路径归一化为绝对路径，末尾追加分隔符，防止前缀匹配攻击（如 `/tmp/foo` 不应匹配 `/tmp/foobar`）。
- **路径验证** (`validatePath`) — 转绝对路径 → 检查允许目录 → 解析符号链接 → 重新检查解析后的路径。对于新文件，验证父目录。
- **符号链接安全** — `buildTree` 和 `validatePath` 均解析符号链接并验证目标在允许目录内。

## 工具处理器（14 个工具）

| 分类 | 工具 |
|----------|-------|
| 文件 I/O | `read_file`, `read_multiple_files`, `write_file`, `copy_file`, `move_file`, `delete_file`, `modify_file` |
| 目录 | `list_directory`, `create_directory`, `tree` |
| 搜索 | `search_files`（glob 文件名匹配）, `search_within_files`（内容子串搜索） |
| 信息 | `get_file_info`, `list_allowed_directories` |

资源处理器：`file://` URI 方案，支持 MIME 类型检测、大小限制、二进制文件 base64 编码。

## 关键常量（`handler/helper.go`）

- `MAX_INLINE_SIZE` = 5MB — 超过此大小的文件返回资源引用，不内联
- `MAX_BASE64_SIZE` = 1MB — 超过此大小的二进制文件返回引用
- `MAX_SEARCHABLE_SIZE` = 10MB — `search_within_files` 跳过超过此大小的文件
- `MAX_SEARCH_RESULTS` = 1000 — 内容搜索结果上限

## 处理器模式

所有处理器遵循一致的模式：
1. 用 `request.RequireString("path")` 提取路径参数
2. 处理 `.` / `./` 相对路径（转为当前工作目录的绝对路径）
3. 用 `validatePath()` 验证路径在允许目录内
4. 执行操作，返回 `mcp.CallToolResult`
5. 错误以 `result.IsError = true` + 文本描述的形式返回，而非直接返回 Go error

## MIME 检测

使用 `github.com/gabriel-vasile/mimetype` 库。三个工具函数：
- `detectMimeType` — 库检测 + 扩展名回退
- `isTextFile` — 判断是否为文本文件（text/* + 常见 application 类型如 json, xml, yaml 等）
- `isImageFile` — 判断是否为图片（image/* 前缀）

## 目录树构建（`buildTree`）

递归目录遍历，可配置最大深度。返回 `FileNode` JSON 树。`followSymlinks` 参数控制是否跟随符号链接（默认 false）。指向允许目录外的符号链接会被跳过。

## 测试

- **包内测试**（`handler/*_test.go`）— 直接调用处理器方法，测试读写/验证/搜索逻辑。
- **外部包测试**（`filesystemserver/*_test.go` 在 `filesystemserver_test` 包中）— 集成测试，通过 `client.NewInProcessClient` 创建进程内 MCP 客户端。
- **辅助函数**（`utils_test.go`）— `startTestClient()` 创建并初始化 MCP 客户端；`getTool()` 按名称从服务器获取工具定义。
- 测试使用 `t.TempDir()` 创建临时目录，`testify`（`assert`/`require`）进行断言。

## Docker

多阶段构建（`golang:1.23-alpine` → `alpine:latest`）。默认 CMD 传入 `/app` 作为允许目录。构建时通过 `-ldflags="-s -w"` 去除调试符号。
