# LLM-Proxy 测试脚本

本目录包含 LLM-Proxy 的 Python 测试脚本。

## 目录结构

```
scripts/
├── venv/                # Python 虚拟环境（不提交到 Git）
├── requirements.txt     # Python 依赖
├── e2e-test.py          # 端到端测试
├── protocol-test.py     # 协议测试
├── debug-test.py        # 调试测试
└── README.md            # 本文档
```

## 快速开始

### 1. 创建虚拟环境

```bash
cd scripts
python -m venv venv
```

### 2. 激活虚拟环境

```bash
# Windows (PowerShell)
.\venv\Scripts\Activate.ps1

# Windows (CMD)
.\venv\Scripts\activate.bat

# Linux/macOS
source venv/bin/activate
```

### 3. 运行测试

```bash
# 返回项目根目录
cd ..

# 使用快速启动脚本（推荐）
python run_tests.py --health

# 或直接运行测试脚本
python scripts/e2e-test.py --all
python scripts/protocol-test.py --openai
```

## 测试脚本说明

### e2e-test.py - 端到端测试

测试 LLM-Proxy 的核心功能：

```bash
python scripts/e2e-test.py --all              # 运行所有测试
python scripts/e2e-test.py --health           # 仅健康检查
python scripts/e2e-test.py --normal           # 仅正常请求测试
python scripts/e2e-test.py --streaming        # 仅流式请求测试
python scripts/e2e-test.py -v                 # 详细输出
```

### protocol-test.py - 协议测试

测试 OpenAI 和 Anthropic 协议的透传和转换：

```bash
python scripts/protocol-test.py               # 运行所有协议测试
python scripts/protocol-test.py --openai      # 仅 OpenAI 协议测试
python scripts/protocol-test.py --anthropic   # 仅 Anthropic 协议测试
python scripts/protocol-test.py --conversion  # 仅协议转换测试
python scripts/protocol-test.py -v            # 详细输出
```

### debug-test.py - 调试测试

最小化测试脚本，用于快速验证环境和连接：

```bash
python scripts/debug-test.py
```

## 注意事项

1. **虚拟环境位置**：`scripts/venv/` 已添加到 `.gitignore`，不会提交到 Git
2. **Python 版本**：需要 Python 3.8 或更高版本
3. **服务地址**：测试脚本默认连接 `http://198.18.0.1:8765`
4. **依赖**：当前测试脚本仅使用 Python 标准库，无需额外依赖

## 故障排除

### 虚拟环境未找到

确保在 `scripts/` 目录下创建了虚拟环境：

```bash
cd scripts
python -m venv venv
```

### 连接失败

确认服务已启动：

```bash
curl http://198.18.0.1:8765/health
```

### 编码错误

测试脚本已配置 UTF-8 编码，如果仍有问题，请确保终端支持 UTF-8。

## 更多信息

详细文档请参考项目根目录的 [VENV_SETUP.md](VENV_SETUP.md)。
