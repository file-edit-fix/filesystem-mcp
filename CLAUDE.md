@AGENTS.md

## Claude Code 特有说明

### MCP 服务器注册

在 `.claude.json` 中注册（Windows 用 `AUTO`）：

```json
{
  "mcpServers": {
    "filesystem": {
      "type": "stdio",
      "command": "filesystem-mcp",
      "args": ["AUTO"]
    }
  }
}
```

### `modify_file` 转义行为

- **`find`**（精确匹配模式）：支持 `interpretEscapeSequences` 转义序列（`\n`、`\r`、`\t`、`\\`）。`\t` 会被解释为实际 Tab 字符。
- **`replace`**：按字面量写入文件（WYSIWYG）。JSON 解码后的字节原样写入。
  - 要写入实际换行：JSON 中使用 `\n`（被解码为换行符）
  - 要写入实际制表符：JSON 中使用 `\t`（被解码为 Tab）
  - 要写入 Go 字面量 `\n`：JSON 中使用 `\\n`（被解码为反斜杠+n）
- **`regex` 模式**：`find` 由 Go `regexp.Compile` 原生处理 `\t`/`\n`/`\d` 等；`replace` 同样按字面量写入。
