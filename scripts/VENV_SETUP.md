# LLM-Proxy Python 测试环境

本项目使用 Python 虚拟环境管理测试脚本依赖，venv 位于 `scripts/` 目录下。

## 环境设置

### 1. 创建虚拟环境

```bash
# 进入 scripts 目录
cd scripts

# 创建虚拟环境
python -m venv venv
```

### 2. 激活虚拟环境

```bash
# Windows (PowerShell)
cd scripts
.\venv\Scripts\Activate.ps1

# Windows (CMD)
cd scripts
.\venv\Scripts\activate.bat

# Linux/macOS
cd scripts
source venv/bin/activate
```

### 3. 安装依赖

```bash
# 激活虚拟环境后，在 scripts 目录安装依赖
cd scripts
pip install -r requirements.txt
cd ..
```

**注意**：当前测试脚本仅使用 Python 标准库，无需安装额外依赖。

## 运行测试

激活虚拟环境后，运行测试脚本：

```bash
# 端到端测试
python scripts/e2e-test.py

# 协议测试
python scripts/protocol-test.py

# 调试测试
python scripts/debug-test.py
```

或使用项目根目录的快速启动脚本：

```bash
python run_tests.py
```

### 测试选项

#### E2E 测试
```bash
python scripts/e2e-test.py --all              # 运行所有测试
python scripts/e2e-test.py --health           # 仅健康检查
python scripts/e2e-test.py --normal           # 仅正常请求测试
python scripts/e2e-test.py --streaming        # 仅流式请求测试
python scripts/e2e-test.py --protocol         # 仅协议测试
python scripts/e2e-test.py -v                 # 详细输出
```

#### 协议测试
```bash
python scripts/protocol-test.py               # 运行所有协议测试
python scripts/protocol-test.py --openai      # 仅 OpenAI 协议测试
python scripts/protocol-test.py --anthropic   # 仅 Anthropic 协议测试
python scripts/protocol-test.py --conversion  # 仅协议转换测试
python scripts/protocol-test.py -v            # 详细输出
```

## 退出虚拟环境

```bash
deactivate
```

## 目录结构

```
llm-proxy/
├── scripts/                   # 测试脚本目录
│   ├── venv/                 # Python 虚拟环境（不提交到 Git）
│   ├── requirements.txt      # Python 依赖
│   ├── e2e-test.py           # 端到端测试
│   ├── protocol-test.py      # 协议测试
│   ├── debug-test.py         # 调试测试
│   └── README.md             # scripts 目录说明
├── run_tests.py              # 快速启动脚本
└── VENV_SETUP.md             # 本文档
```

## 注意事项

1. **虚拟环境不提交到 Git**：`scripts/venv/` 目录已添加到 `.gitignore`
2. **Python 版本要求**：Python 3.8 或更高版本
3. **Windows 编码问题**：测试脚本已配置 UTF-8 编码，确保正确显示中文和特殊字符
4. **服务地址**：测试脚本默认连接 `http://198.18.0.1:8765`，请确保服务已启动

## 故障排除

### 激活脚本执行策略错误（Windows）

如果遇到 "无法加载文件，因为在此系统上禁止运行脚本" 错误：

```powershell
Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser
```

### 编码错误

如果看到乱码或编码错误，确保：
1. 终端支持 UTF-8 编码
2. Python 脚本已包含 `sys.stdout.reconfigure(encoding='utf-8')`

### 连接失败

如果测试无法连接到服务：
1. 确认服务已启动：`curl http://198.18.0.1:8765/health`
2. 检查防火墙设置
3. 验证配置文件中的监听地址
