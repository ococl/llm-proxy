# 系统提示词注入功能设计文档 (v2)

## 1. 概述

为 LLM Proxy 增加系统提示词注入功能。支持从 `system_prompt.md` 文件或 `system_prompts/` 目录加载提示词，注入到 OpenAI 兼容接口的 `messages` 数组中。

## 2. 详细规范

### 2.1 文件来源

| 来源 | 路径 | 说明 |
|------|------|------|
| 单文件 | `system_prompt.md` | 可执行文件同目录 |
| 多文件 | `system_prompts/*.md` | 可执行文件同目录下的子目录 |

### 2.2 加载顺序

1. 按 `priority` 字段排序（数值小的优先）
2. 相同 priority 按文件名字母排序
3. 每个文件独立配置，独立判断是否注入

### 2.3 文件格式

采用 YAML Front Matter + Markdown 内容格式。

```markdown
---
position: before
separator: double-newline
models: ["*"]
enabled: true
priority: 100
---

你的系统提示词内容...
```

### 2.4 配置字段 (Metadata)

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `position` | string | `before` | 注入位置：`before` 或 `after` |
| `separator` | string | `double-newline` | 分隔符：`newline`, `double-newline`, `none`, `custom` |
| `custom_separator` | string | `""` | 当 `separator` 为 `custom` 时使用 |
| `models` | []string | `["*"]` | 支持通配符的模型匹配列表 |
| `enabled` | bool | `true` | 是否启用 |
| `priority` | int | `100` | 加载优先级（数值小的优先） |

### 2.5 注入逻辑 (Position)

- `before`: 注入内容放在原 system message 前面，合并为一条
- `after`: 注入内容放在原 system message 后面，合并为一条

### 2.6 内置变量

使用 `_` 前缀区分内置变量和环境变量。

| 类别 | 变量 | 说明 |
|------|------|------|
| **系统** | `${_OS}` | 操作系统 (windows/linux/darwin) |
| | `${_ARCH}` | CPU 架构 (amd64/arm64) |
| | `${_HOSTNAME}` | 主机名 |
| | `${_USER}` | 当前用户名 |
| | `${_HOME}` | 用户主目录 |
| | `${_PWD}` | 当前工作目录 |
| **开发环境** | `${_SHELL}` | 当前 Shell (bash/zsh/powershell/cmd) |
| | `${_LANG}` | 系统语言 |
| | `${_EDITOR}` | 默认编辑器 |
| | `${_TERM}` | 终端类型 |
| **时间** | `${_DATE}` | 当前日期 (YYYY-MM-DD) |
| | `${_TIME}` | 当前时间 (HH:MM:SS) |
| | `${_DATETIME}` | ISO 格式时间戳 |
| **代理** | `${_PROXY_VERSION}` | llm-proxy 版本号 |
| | `${_MODEL}` | 当前请求的模型名 |

### 2.7 环境变量替换

支持 `${VAR}` 或 `${VAR:-default}` 语法。

## 3. 使用示例

### 单文件示例

```markdown
---
position: before
separator: double-newline
models: ["claude-*", "gpt-4*"]
---

## 当前环境
- 操作系统: ${_OS} (${_ARCH})
- Shell: ${_SHELL}
- 当前时间: ${_DATETIME}

请根据操作系统选择合适的命令格式。
```

### 多文件示例

```
system_prompts/
├── 01_base.md      (priority: 10)
├── 02_security.md  (priority: 20)
└── 03_coding.md    (priority: 30)
```

## 4. 测试覆盖

- ✅ 配置解析测试
- ✅ 环境变量替换测试
- ✅ 内置变量替换测试
- ✅ 模型匹配测试
- ✅ 注入位置测试 (before/after)
- ✅ 多文件加载和排序测试
