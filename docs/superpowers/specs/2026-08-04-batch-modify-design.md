# 跨文件批量修改设计

## 背景

Issue #6.2：`go vet` 一次输出所有文件的未使用 import，但 `modify_file` 一次只能处理一个文件，导致循环执行 vet → modify → vet → modify 的繁琐流程。

## 目标

新增 `paths` 参数，让 `modify_file` 支持对多个文件执行相同的查找替换，返回汇总结果。

## 参数设计

| 参数 | 类型 | 必须 | 说明 |
|------|------|------|------|
| `path` | string | 否 | 单文件路径（保留，向后兼容） |
| `paths` | string[] | 否 | 文件路径数组（新增） |
| `find` | string | 是 | 查找文本 |
| `replace` | string | 是 | 替换文本 |
| `regex` | bool | 否 | 是否为正则（默认 false） |
| `all_occurrences` | bool | 否 | 替换全部匹配（默认 true） |
| `dry_run` | bool | 否 | 预览模式（默认 false） |

**规则：**
- `paths` 和 `path` 同时提供 → 优先使用 `paths`，忽略 `path`
- 两者都未提供 → 报错 "Error: either path or paths must be specified"
- 空数组 → 报错 "Error: paths array must not be empty"

## 执行逻辑

对 `paths` 中的每个文件独立执行：
1. 路径安全验证（`validatePath`）
2. 文件读取 + CRLF 归一化
3. 精确匹配 / 正则匹配
4. 空白行归并（4+ → 2）
5. 原子写入（tmp + rename）

每个文件独立处理，一个文件失败不影响其他文件。

## 返回格式

dry_run 模式：
```
Dry run: 3 file(s), 12 match(es) found

file1.go (line 5):
  import "encoding/json"
  -> would be replaced with: (empty, line removed)

file2.go: No matches
file3.go (line 12):
  ...
```

实际执行模式：
```
Batch modify completed: 2/3 files modified

  health_check_test.go: 5 replacement(s)
  config_test.go: 3 replacement(s)
  utils.go: Error — file not found
```

## 约束

- 每个文件独立原子写入，失败不影响其他文件
- 路径安全验证沿用现有 `validatePath` 逻辑
- 不改变任何现有行为，纯增量

## 测试计划

- 多文件全部成功
- 多文件部分成功（某文件不存在）
- 多文件全部无匹配
- 空 paths 数组报错
- 未提供 path/paths 报错
- dry_run 汇总所有文件匹配结果
