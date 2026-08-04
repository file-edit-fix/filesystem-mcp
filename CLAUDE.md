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

### `modify_file` 转义序列

当 `regex: true` 时，`replace` 参数中的 `\t`、`\n`、`\r`、`\` 会被 `interpretEscapeSequences()` 解释为实际字符，而非字面量。这是 `modify_file` 区别于 Go 标准库 `regexp.ReplaceAllString` 的关键特性。
