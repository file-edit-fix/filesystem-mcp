# 转义链简化设计：移除 `replace` 的 `interpretEscapeSequences`

## 背景

### 问题

`modify_file` 的 `replace` 参数经历了**双重转义**，导致 LLM 无法预测输出结果：

1. **JSON 层**：`\\n` 被解码为 `\n`（反斜杠 + n）
2. **`interpretEscapeSequences` 层**：`\n` 再被解码为真实换行符（0x0A）

```
LLM 在 tool call 中写:   "\\\\n"  (4 个反斜杠 + n)
  ↓ JSON 解码
Go 收到:                "\\n"    (2 个反斜杠 + n)
  ↓ interpretEscapeSequences
文件写入:               "\n"     (1 个反斜杠 + n) ← 这才是 LLM 想要的
```

但 LLM 的直觉（来自内置 Edit 工具的 WYSIWYG 经验）是"写 `\\n` 得到 `\n`"：

```
LLM 认为: 写 "\\n" → JSON 解码 → "\n" (反斜杠+n) → 写入文件 → "\n"
实际发生: 写 "\\n" → JSON 解码 → "\n" → interpretEscapeSequences → 真实换行
```

### 触发场景

- 写入 Go 字符串字面量中的 `\n`、`\t` 等转义序列
- 写入正则表达式中的 `\d`、`\w` 等
- 写入 JSON 模板中的 `\"`、`\\` 等
- 任何需要在文件中保留反斜杠序列的场景

### 关联 Issue

- **filesystem-mcp #48** — 非 regex 模式 replace 中 JSON 转义链不可预测，from AK-Switch
- 关联 filesystem-mcp #2（regex 模式下 `\n` 被当作字面量）

## 根因分析

`interpretEscapeSequences()` 在 `replace` 上的应用是多余的。JSON 解码层已经提供了必要的转义能力：

| JSON 写法 | JSON 解码后 | 期望文件内容 |
|-----------|-------------|------------|
| `\t` | 实际 Tab | 实际 Tab |
| `\n` | 实际换行 | 实际换行 |
| `\\` | `\`（单个反斜杠） | `\` |
| `\\n` | `\n`（反斜杠+n） | `\n` 转义序列 |

JSON 解码后的结果已经是"文件应该写入的字节"。`interpretEscapeSequences` 在此基础上又做了一层相同的转义，导致 LLM 必须写 `\\\\n` 才能得到 `\n`。

## 设计决策

### 核心变更

**移除 `replace` 参数上的 `interpretEscapeSequences()` 调用。**

受影响的位置：

- `mixedReplace()`（非 regex 模式）：`modify_file.go:260`
- `regexReplace()`（regex 模式）：`modify_file.go:350`

### 保留 `find` 上的 `interpretEscapeSequences`

`find` 参数保留 `interpretEscapeSequences`，原因：

- LLM 在 JSON 中写 `\t`（实际 Tab 字符）即可匹配文件中的 Tab，无需额外工具
- 但 `interpretEscapeSequences` 提供了第二层保护：如果 LLM 写的是 `\t` 字面量（反斜杠+t），也能正确匹配文件中的 Tab
- `find` 没有"JSON 已经够用"的问题（LLM 经常不确定自己写的是实际 Tab 还是反斜杠+t）

### 统一 regex 和非 regex 模式

当前差异：

| 模式 | `find` 转义 | `replace` 转义 |
|------|------------|---------------|
| 非 regex | `interpretEscapeSequences` | `interpretEscapeSequences`（移除） |
| regex | Go `regexp.Compile` 原生处理 | `interpretEscapeSequences`（移除） |

变更后，`replace` 在两个模式下行为一致：**按字面量写入**。

regex 模式下的 `replace` 与 Go 标准库 `regexp.ReplaceAllString` 行为一致——`\t` 在替换字符串中按字面量处理。

## 代码变更

### 1. `modify_file.go` — 核心逻辑

**`mixedReplace()` 函数**（行 258-261）：

```go
// 变更前
normalizedFind := interpretEscapeSequences(strings.ReplaceAll(find, "\r\n", "\n"))
normalizedReplace := interpretEscapeSequences(replace)

// 变更后
normalizedFind := interpretEscapeSequences(strings.ReplaceAll(find, "\r\n", "\n"))
normalizedReplace := replace  // 直接使用，不再经过转义解释
```

**`regexReplace()` 函数**（行 338-350）：

```go
// 变更前
normalizedReplace := interpretEscapeSequences(replace)

// 变更后
normalizedReplace := replace  // 直接使用，不再经过转义解释
```

### 2. `CLAUDE.md` — 文档更新

当前行 21-23：

```
### `modify_file` 转义序列

当 `regex: true` 时，`replace` 参数中的 `\t`、`\n`、`\r`、`\` 会被 `interpretEscapeSequences()` 解释为实际字符，而非字面量。
```

改为：

```
### `modify_file` 转义行为

- **`find`**（精确匹配模式）：支持 `interpretEscapeSequences` 转义序列（`\n`、`\r`、`\t`、`\\`）。`\t` 在 find 中会被解释为实际 Tab 字符。
- **`replace`**：按字面量写入文件（WYSIWYG）。JSON 解码后的字节原样写入。
  - 要写入实际换行：JSON 中使用 `\n`（被解码为换行符）
  - 要写入实际制表符：JSON 中使用 `\t`（被解码为 Tab）
  - 要写入 Go 字面量 `\n`：JSON 中使用 `\\n`（被解码为反斜杠+n）
- **`regex` 模式**：`find` 由 Go `regexp.Compile` 原生处理 `\t`/`\n`/`\d` 等标准正则转义；`replace` 同样按字面量写入。
```

### 3. `CONTEXT.md` — 术语更新

行 15 改为：

```
- **escape sequence interpretation** — The `interpretEscapeSequences()` function that translates `\n`, `\r`, `\t`, `\` in the **find** string to actual characters before matching. Does NOT apply to the `replace` string, which is written literally.
```

### 4. `AGENTS.md` — 更新 CODE STYLE 部分

当前行 28-29：

```
- **工具参数**：`modify_file` 的 `find`（精确匹配模式）和 `replace` 均支持转义序列
```

改为：

```
- **工具参数**：`modify_file` 的 `find`（精确匹配模式）支持转义序列（`\n`、`\t`、`\r`、`\\`）；`replace` 按字面量写入文件，不支持转义序列解释
```

## 测试变更

### 需修改的测试

| 测试名 | 当前 `replace` 值 | 修改后 | 原因 |
|--------|-------------------|--------|------|
| `TestModifyFile_RegexReplaceWithEscapedNewline` | `"\\n"` | `"\n"` | 用 Go 编译器转义获得实际换行，不再依赖 `interpretEscapeSequences` |
| `TestModifyFile_ExactReplaceWithEscapedNewline` | `"\\n"` | `"\n"` | 同上 |
| `TestModifyFile_RegexReplaceBackslashLiteral` | `"D:\\\\Users\\\\newuser"` | `"D:\\Users\\newuser"` | 用 Go 编译器转义获得单反斜杠 |
| `TestModifyFile_ExactReplaceWithEscapedBackslash` | `"D:\\\\Users\\\\newuser"` | `"D:\\Users\\newuser"` | 同上 |

### 不受影响的测试

**所有使用 `\t`（Tab 字符）的测试**不受影响——Go 编译器已将 `\t` 编译为实际 Tab：

- `TestModifyFile_RegexReplaceEscapeSequences`
- `TestModifyFile_ExactReplaceWithEscapedTab`
- `TestModifyFile_RegexFindWithEscapedTab`（`find` 保留转义）
- `TestModifyFile_ExactFindWithEscapedTab`（`find` 保留转义）

**所有使用 `\n`（实际换行符）的测试**不受影响——Go 编译器已将 `\n` 编译为实际换行：

- `TestModifyFile_RegexReplaceNewline`
- `TestModifyFile_ExactFindWithEscapedNewline`（`find` 保留转义）

**所有 CRLF 测试**不受影响。

### 新增测试

1. **`TestModifyFile_ReplaceWritesLiteralBackslashN`** — 验证非 regex 模式下 `replace` 中的 `\n`（反斜杠+n）按字面量写入：
   - 输入：文件含 `prefix suffix`
   - `replace: "\\n"`（Go 中反斜杠+n）
   - 期望：文件变为 `prefix\n suffix`（实际反斜杠+n 字符，而非换行）

2. **`TestModifyFile_ReplaceWritesLiteralBackslashT`** — 验证 regex 模式下 `replace` 中的 `\t` 按字面量写入：
   - 输入：文件含 `foo123bar`
   - `regex: true`，`find: "[0-9]+"`，`replace: "\\t"`（Go 中反斜杠+t）
   - 期望：`foo\tbar`

## 边界情况

- **空 `replace`**：正常写入空字符串，匹配到的内容被删除
- **`replace` 含 `$1` 反向引用**：当前实现按字节偏移手动替换，不通过 `regexp.ReplaceAllString`，所以 `$1` 在两种模式下均按字面量写入。这是一个已知限制，不在本次范围内。
- **批量模式**：`paths` 参数同样遵循新语义，无额外变更

## 风险与缓解

| 风险 | 影响 | 缓解 |
|------|------|------|
| 现有用户依赖 `\t`→Tab 行为 | 低 — 实际测试表明 `\t` 在 Go 源码中已被编译器转义为实际 Tab | 确保测试覆盖 |
| 现有用户依赖 `\\n`→换行行为 | 中 — 需要修改测试中的 `\\n` 为 `\n` | 设计文档已列出所有受影响测试 |
| 回归导致 `find` 匹配失败 | 低 — `find` 逻辑不变 | 现有 find 测试全部保留 |
| 双反斜杠路径写入 | 低 — 用户可直接在 JSON 中写 `\\` 获得单个 `\` | 测试覆盖 |

## 验证

1. 运行 `go test ./...` 所有测试通过
2. 手动构建 `go build -o server .` 成功
3. 运行 `go vet ./...` 无警告
4. 新增测试验证 WYSIWYG 语义