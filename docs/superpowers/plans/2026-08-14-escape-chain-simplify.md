# 转义链简化 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 移除 `modify_file` 的 `replace` 参数上的 `interpretEscapeSequences` 调用，使 replace 按字面量（WYSIWYG）写入文件，统一 regex/非 regex 模式行为，解决 JSON 双重转义导致的 LLM 转义链不可预测问题（#48）。

**Architecture:** `replace` 字符串在 JSON 解码后直接写入文件，不再经过 `interpretEscapeSequences` 二次转义。`find` 参数保留 `interpretEscapeSequences`（匹配字节用）。regex 模式下 `find` 继续由 Go `regexp.Compile` 原生处理。同步更新测试与三份文档。

**Tech Stack:** Go 1.23+ · MCP stdio · `github.com/mark3labs/mcp-go` · testify

## Global Constraints

- [ ] 不改变 CRLF 归一化（`modify_file.go:207` 的 `strings.ReplaceAll("\r\n", "\n")`）
- [ ] `find` 参数的 `interpretEscapeSequences` 必须保留（both mixedReplace 和 regexReplace）
- [ ] 新增行为：`replace` 按字面量写入（WYSIWYG）——JSON 解码后的字节原样进入文件
- [ ] regex 模式下 `replace` 行为与 Go `regexp.ReplaceAllString` 一致（`\t` 按字面量）
- [ ] 所有 `.go` 文件使用 Tab 缩进（Read 输出行号后的第一个 Tab 是列分隔符，old_string 用比 Read 显示的少 1 个前导 Tab）
- [ ] 提交信息格式：`类型: 简短描述`（fix/refactor/docs/test）
- [ ] 每个任务结束时 `go test ./filesystemserver/handler/ -run TestModifyFile` 必须通过

---

### Task 1: 调整现有测试 + 新增 WYSIWYG 测试

**Files:**
- Modify: `filesystemserver/handler/modify_file_test.go`
  - 行 453-482：`TestModifyFile_RegexReplaceWithEscapedNewline` — `replace` 从 `"\\n"` 改为 `"\n"`
  - 行 485-515：`TestModifyFile_ExactReplaceWithEscapedNewline` — `replace` 从 `"\\n"` 改为 `"\n"`
  - 行 332-361：`TestModifyFile_RegexReplaceBackslashLiteral` — `replace` 从 `"D:\\\\Users\\\\newuser"` 改为 `"D:\\Users\\newuser"`
  - 行 675-706：`TestModifyFile_ExactReplaceWithEscapedBackslash` — `replace` 从 `"D:\\\\Users\\\\newuser"` 改为 `"D:\\Users\\newuser"`
  - 文件末尾新增 2 个测试（见 Step 1）

**Interfaces:**
- Consumes: `FilesystemHandler.HandleModifyFile`（现有签名，不变）
- Produces: 2 个新测试名 `TestModifyFile_ReplaceWritesLiteralBackslashN`、`TestModifyFile_ReplaceWritesLiteralBackslashT`，供 Task 2 验证新行为

- [ ] **Step 1: 修改 4 个现有测试的 `replace` 值**

**注意 Read 输出格式**：此文件用 Tab 缩进，Read 行号后的第一个 Tab 是列分隔符。old_string 用比 Read 显示的**少 1 个**前导 Tab。

将 4 处 `replace` 的值改为：

```go
// TestModifyFile_RegexReplaceWithEscapedNewline (行 470)
"replace":         "\n",
// TestModifyFile_ExactReplaceWithEscapedNewline (行 502)
"replace":         "\n",
// TestModifyFile_RegexReplaceBackslashLiteral (行 349)
"replace":         "D:\\Users\\newuser",
// TestModifyFile_ExactReplaceWithEscapedBackslash (行 694)
"replace":         "D:\\Users\\newuser",
```

（Go 源码中 `"\n"` 由编译器转义为实际换行 0x0A；`"D:\\Users\\newuser"` 为单反斜杠。行为不变——直接写入实际字符。）

- [ ] **Step 2: 新增 WYSIWYG 测试**

在文件末尾追加：

```go
// TestModifyFile_ReplaceWritesLiteralBackslashN verifies that a literal
// backslash+n in replace is written as-is (WYSIWYG), not interpreted as a
// newline. Regression test for issue #48.
func TestModifyFile_ReplaceWritesLiteralBackslashN(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.txt")
	originalContent := "prefix suffix"
	err := os.WriteFile(filePath, []byte(originalContent), 0644)
	require.NoError(t, err)

	handler, err := NewFilesystemHandler(resolveAllowedDirs(t, dir))
	require.NoError(t, err)

	// "\\n" in Go source is backslash+n (two chars) — must be written literally
	request := mcp.CallToolRequest{}
	request.Params.Name = "modify_file"
	request.Params.Arguments = map[string]any{
		"path":            filePath,
		"find":            " ",
		"replace":         "\\n",
		"all_occurrences": false,
	}

	result, err := handler.HandleModifyFile(context.Background(), request)
	require.NoError(t, err)
	assert.False(t, result.IsError)

	content, err := os.ReadFile(filePath)
	require.NoError(t, err)
	assert.Equal(t, "prefix\\nsuffix", string(content),
		"literal backslash+n in replace should be written as-is, not decoded to newline")
}

// TestModifyFile_ReplaceWritesLiteralBackslashT verifies that a literal
// backslash+t in replace is written as-is in regex mode (matching Go
// regexp.ReplaceAllString semantics).
func TestModifyFile_ReplaceWritesLiteralBackslashT(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.txt")
	originalContent := "foo123bar"
	err := os.WriteFile(filePath, []byte(originalContent), 0644)
	require.NoError(t, err)

	handler, err := NewFilesystemHandler(resolveAllowedDirs(t, dir))
	require.NoError(t, err)

	request := mcp.CallToolRequest{}
	request.Params.Name = "modify_file"
	request.Params.Arguments = map[string]any{
		"path":            filePath,
		"find":            "[0-9]+",
		"replace":         "\\t",
		"regex":           true,
		"all_occurrences": true,
	}

	result, err := handler.HandleModifyFile(context.Background(), request)
	require.NoError(t, err)
	assert.False(t, result.IsError)

	content, err := os.ReadFile(filePath)
	require.NoError(t, err)
	assert.Equal(t, "foo\\tbar", string(content),
		"regex mode: literal backslash+t in replace should be written as-is")
}
```

- [ ] **Step 3: 运行测试，确认新测试失败、旧测试通过**

Run: `go test ./filesystemserver/handler/ -run TestModifyFile -v 2>&1 | Select-String -Pattern "PASS|FAIL|ReplaceWritesLiteralBackslash"`
Expected:
- `TestModifyFile_ReplaceWritesLiteralBackslashN` **FAIL**（当前代码把 `\n` 解释为换行，期望字面量不匹配）
- `TestModifyFile_ReplaceWritesLiteralBackslashT` **FAIL**（同上，`\t` 被解释为 Tab）
- 其余 `TestModifyFile_*` **PASS**（修改后的 `"\n"`/`"D:\...` 在旧代码下也通过——Go 编译器已转义为实际字符，`interpretEscapeSequences` 原样返回）

> 若 Windows 控制台编码异常，可在 Git Bash 中运行：
> `go test ./filesystemserver/handler/ -run "TestModifyFile_(ReplaceWritesLiteral|Basic)" -v`

- [ ] **Step 4: Commit**

```bash
git add filesystemserver/handler/modify_file_test.go
git commit -m "test(modify_file): 调整转义测试为 WYSIWYG 语义，新增字面量写入测试"
```

---

### Task 2: 移除 `replace` 的 `interpretEscapeSequences`

**Files:**
- Modify: `filesystemserver/handler/modify_file.go`
  - 行 258-261：`mixedReplace()` — `normalizedReplace := replace`
  - 行 338-350：`regexReplace()` — `normalizedReplace := replace`

**Interfaces:**
- Consumes: Task 1 新增的测试 `TestModifyFile_ReplaceWritesLiteralBackslashN` / `_T`
- Produces: 行为变更——`replace` 按字面量写入（无签名变化）

- [ ] **Step 1: 修改 `mixedReplace()`**

```go
// 变更前 (行 258-260)
	normalizedFind := interpretEscapeSequences(strings.ReplaceAll(find, "\r\n", "\n"))
	normalizedReplace := interpretEscapeSequences(replace)

// 变更后
	normalizedFind := interpretEscapeSequences(strings.ReplaceAll(find, "\r\n", "\n"))
	normalizedReplace := replace
```

**只改这一行**，`normalizedFind` 不动。

- [ ] **Step 2: 修改 `regexReplace()`**

```go
// 变更前 (行 350)
	normalizedReplace := interpretEscapeSequences(replace)

// 变更后
	normalizedReplace := replace
```

- [ ] **Step 3: 运行全部测试**

Run: `go test ./filesystemserver/handler/ -run TestModifyFile`
Expected: 全部 PASS，包括 Task 1 新增的 2 个字面量测试

- [ ] **Step 4: 检查 `interpretEscapeSequences` 是否还有调用点**

Run: `go vet ./filesystemserver/handler/modify_file.go`
说明：`interpretEscapeSequences` 函数本身保留（`find` 仍使用），只是 `replace` 不再调用。用 grep 确认调用点只剩 `find`：
```bash
grep -n "interpretEscapeSequences" filesystemserver/handler/modify_file.go
```
Expected: 只剩 `mixedReplace` 中 `find` 一处调用（行 259）。`regexReplace` 中不再有调用。

- [ ] **Step 5: Commit**

```bash
git add filesystemserver/handler/modify_file.go
git commit -m "fix(modify_file): replace 按字面量写入，移除 interpretEscapeSequences 二次转义 (#48)"
```

---

### Task 3: 更新文档

**Files:**
- Modify: `CLAUDE.md`（行 21-23）
- Modify: `CONTEXT.md`（行 15）
- Modify: `AGENTS.md`（行 28-29）

- [ ] **Step 1: 更新 `CLAUDE.md`**

将行 21-23：

```markdown
### `modify_file` 转义序列

当 `regex: true` 时，`replace` 参数中的 `\t`、`\n`、`\r`、`\` 会被 `interpretEscapeSequences()` 解释为实际字符，而非字面量。这是 `modify_file` 区别于 Go 标准库 `regexp.ReplaceAllString` 的关键特性。
```

改为：

```markdown
### `modify_file` 转义行为

- **`find`**（精确匹配模式）：支持 `interpretEscapeSequences` 转义序列（`\n`、`\r`、`\t`、`\\`）。`\t` 会被解释为实际 Tab 字符。
- **`replace`**：按字面量写入文件（WYSIWYG）。JSON 解码后的字节原样写入。
  - 要写入实际换行：JSON 中使用 `\n`（被解码为换行符）
  - 要写入实际制表符：JSON 中使用 `\t`（被解码为 Tab）
  - 要写入 Go 字面量 `\n`：JSON 中使用 `\\n`（被解码为反斜杠+n）
- **`regex` 模式**：`find` 由 Go `regexp.Compile` 原生处理 `\t`/`\n`/`\d` 等；`replace` 同样按字面量写入。
```

- [ ] **Step 2: 更新 `CONTEXT.md`**

行 15 改为：

```markdown
- **escape sequence interpretation** — The `interpretEscapeSequences()` function that translates `\n`, `\r`, `\t`, `\` in the **find** string to actual characters before matching. Does NOT apply to the `replace` string, which is written literally.
```

- [ ] **Step 3: 更新 `AGENTS.md`**

行 28-29：

```markdown
- **工具参数**：`modify_file` 的 `find`（精确匹配模式）支持转义序列（`\n`、`\t`、`\r`、`\\`）；`replace` 按字面量写入文件，不支持转义序列解释
```

（把"和 `replace` 均支持转义序列"改为"支持转义序列…；`replace` 按字面量写入文件，不支持转义序列解释"）

- [ ] **Step 4: Commit**

```bash
git add CLAUDE.md CONTEXT.md AGENTS.md
git commit -m "docs: 更新 modify_file 转义行为文档（replace WYSIWYG）"
```

---

### Task 4: 全量验证

**Files:**
- 无修改（仅验证）

- [ ] **Step 1: 运行完整测试套件**

Run: `go test ./...`
Expected: 所有包全部 PASS（修改文件相关 + 其他工具不受影响）

- [ ] **Step 2: 静态分析**

Run: `go vet ./...`
Expected: 无输出（通过）

- [ ] **Step 3: 构建**

Run: `go build -o server .`
Expected: 生成 `server` 二进制，无错误

- [ ] **Step 4: 手动验证 WYSIWYG 行为（可选但推荐）**

在临时目录创建测试文件并用 dry_run 验证：

```bash
# 创建临时测试文件
echo 'w.Write([]byte("data" + "\n"))' > /tmp/esc_test.txt
# 启动服务器
./server /tmp
```

用 MCP 客户端调用 `modify_file`：
- `find: "\n"`（实际换行，或 `\\n` 转义序列）
- `replace: "\\n"`（JSON 中反斜杠+n）
- `dry_run: true`

预期：dry run 输出显示替换为字面量 `\n`（可以通过输出内容确认——若被解释为换行则是 bug）

> 若不方便启动 MCP 客户端，可跳过此步骤，Step 1-3 已足够。

- [ ] **Step 5: 确认分支状态并准备 PR**

```bash
git log --oneline -5
git status
```

Expected: 3 个提交（test / fix / docs），工作树干净。若完成，转 PR 流程。

---

## 自检记录（writing-plans skill 要求）

**Spec 覆盖：**
- ✅ 核心变更（移 replace 的 interpretEscapeSequences）→ Task 2
- ✅ 保留 find 转义 → Task 2 Step 4 验证（grep 确认 find 仍保留）
- ✅ 统一 regex/非 regex → Task 2（两处都改）
- ✅ 测试：4 个调整 + 2 个新增 → Task 1
- ✅ 文档：CLAUDE.md / CONTEXT.md / AGENTS.md → Task 3
- ✅ 边界/风险（空 replace、批量模式无额外变更）→ 无需代码，Task 4 全量测试兜底

**占位符扫描：** 无 TBD/TODO/vague 步骤；每步含实际代码或精确命令。

**类型一致性：** 测试用 `handler.HandleModifyFile` 签名与 Task 1 一致；新增测试名在 Task 1 定义、Task 2 引用，命名一致。