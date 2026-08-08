# filesystem-mcp — AGENTS.md

Fork 自 [mark3labs/mcp-filesystem-server](https://github.com/mark3labs/mcp-filesystem-server)，作为 Claude Code Windows Edit 工具的后备方案。核心新增是 `modify_file` 工具——在文件系统层面做查找替换，不经过 Edit 工具链，无 CRLF 哈希检查 bug。

## Tech Stack

Go 1.23+ · MCP stdio 协议 · `github.com/mark3labs/mcp-go`

## Git Workflow

本仓库使用 **GitHub Flow**：从 main 创建功能分支，通过 PR 合并，合并后删除分支。

## Dev Environment Tips

```bash
# 全局安装（到 ~/go/bin/）
go install .

# 调试运行
./server /path/to/allowed/dir
# Windows: AUTO 展开为所有可用驱动器
./server AUTO
```

## Build & Test

| 命令 | 用途 |
|------|------|
| `go build -o server .` | 构建二进制 |
| `go test ./...` | 全部测试 |
| `go test -race ./...` | 竞争检测 |
| `go vet ./...` | 静态分析 |
| `go mod tidy` | 整理依赖 |

## Project Structure

```
filesystem-mcp/
├── main.go                        ← 入口：解析 CLI 参数（AUTO 关键字），stdio 服务
├── filesystemserver/
│   ├── server.go                  ← NewFilesystemServer()，注册 14 个工具
│   ├── handler/
│   │   ├── handler.go             ← FilesystemHandler 结构体，访问控制
│   │   ├── helper.go              ← 路径验证、MIME 检测、buildTree、resolvePath
│   │   ├── modify_file.go         ← 核心：精确匹配 + 正则 + 行级模糊匹配
│   │   ├── read_file.go           ← 原始字节读取，无任何归一化
│   │   ├── write_file.go          ← 原子写入
│   │   ├── search_files.go        ← 文件名 glob 搜索
│   │   ├── search_within_files.go ← 文件内容搜索
│   │   ├── list_directory.go      ← 目录列表
│   │   ├── tree.go                ← 目录树
│   │   ├── get_file_info.go       ← 文件信息（时间戳）
│   │   ├── copy_file.go / move_file.go / delete_file.go / create_directory.go
│   │   ├── read_multiple_files.go ← 批量读取
│   │   ├── resources.go           ← file:// 资源处理器
│   │   ├── types.go               ← 共享类型和常量
│   │   └── *_test.go              ← 每个工具的测试
│   └── utils_test.go              ← 测试辅助函数
```

## Code Style & Conventions

- **匹配逻辑**：`mixedReplace()` — 精确匹配优先，失败后回退到行级 trim 模糊匹配
- **CRLF 处理**：读入时统一 `\r\n` → `\n`，这是设计意图不是 bug（见 `modify_file.go:207`）
- **工具参数**：`modify_file` 的 `find`（精确匹配模式）和 `replace` 均支持转义序列（`\n`、`\t`、`\r`、`\\`）；`regex` 模式由 Go `regexp.Compile` 原生处理 `\t`/`\n`/`\r`
- **错误处理**：返回 `*mcp.CallToolResult` + error，`IsError` 标记错误响应

## Key Constants (`types.go`)

| 常量 | 值 | 用途 |
|------|----|------|
| `MAX_INLINE_SIZE` | 5MB | 超过此大小的文件返回资源引用 |
| `MAX_BASE64_SIZE` | 1MB | 二进制文件 base64 上限 |
| `MAX_SEARCHABLE_SIZE` | 10MB | 内容搜索跳过超限文件 |
| `MAX_SEARCH_RESULTS` | 1000 | 搜索结果上限 |

## Security Model

- **允许目录** — 启动时指定，路径末尾追加分隔符防止前缀匹配攻击
- **符号链接** — `validatePath` 和 `buildTree` 均解析符号链接并验证目标在允许目录内
- **新文件写入** — 验证父目录在允许目录内

## Boundaries

- ✅ **Always**: 添加/修复工具逻辑、修改测试、更新 MCP 工具 schema
- ⚠️ **Ask first**: 修改安全模型（允许目录验证逻辑）、增加新 MCP 工具 schema
- 🚫 **Never**: 改变 CRLF 归一化行为（`strings.ReplaceAll("\r\n", "\n")`），这会破坏匹配语义

## Reference

详细架构和安全模型见 `CLAUDE.md`。
