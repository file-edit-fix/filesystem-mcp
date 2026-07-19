# Filesystem MCP Server

> 本项目是 [mark3labs/mcp-filesystem-server](https://github.com/mark3labs/mcp-filesystem-server) 的个人自用 Fork。不开源、不接受 PR，仅为本人的 Claude Code 环境提供文件系统访问能力。

基于 MCP (Model Context Protocol) 协议的文件系统访问服务器，提供 14 个工具和 1 个 `file://` 资源处理器。

## 组件

### 工具

#### 文件操作

- **read_file** — 读取文件完整内容
- **read_multiple_files** — 批量读取多个文件
- **write_file** — 创建或覆盖文件
- **copy_file** — 复制文件或目录
- **move_file** — 移动或重命名文件/目录
- **delete_file** — 删除文件或目录（支持递归）
- **modify_file** — 查找替换文本（支持精确匹配和正则）

#### 目录操作

- **list_directory** — 列出目录内容
- **create_directory** — 创建目录
- **tree** — 递归目录树 JSON 输出

#### 搜索与信息

- **search_files** — 按文件名模式搜索
- **search_within_files** — 按文件内容搜索
- **get_file_info** — 获取文件/目录元数据
- **list_allowed_directories** — 列出允许访问的目录

### 使用

```bash
# 构建
go build -o server .

# 运行（指定允许目录）
./server /path/to/allowed/dir

# Windows: AUTO 自动展开所有可用驱动器
./server AUTO
```

### MCP 配置

```json
{
  "mcpServers": {
    "filesystem": {
      "command": "path/to/filesystem-mcp/server",
      "args": ["D:/", "E:/"]
    }
  }
}
```

## License

See the [LICENSE](LICENSE) file for details.