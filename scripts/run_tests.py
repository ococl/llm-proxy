#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
快速启动脚本 - 激活 scripts/venv 并运行测试

使用方法:
    python run_tests.py                    # 运行所有 E2E 测试
    python run_tests.py --health           # 仅健康检查
    python run_tests.py --protocol         # 运行协议测试
    python run_tests.py --all              # 运行所有测试
"""

import os
import sys
import subprocess
from pathlib import Path

PROJECT_ROOT = Path(__file__).parent.resolve()
VENV_PYTHON = PROJECT_ROOT / "scripts" / "venv" / "Scripts" / "python.exe"

if not VENV_PYTHON.exists():
    VENV_PYTHON = PROJECT_ROOT / "scripts" / "venv" / "bin" / "python"

if not VENV_PYTHON.exists():
    print("错误: 虚拟环境未找到")
    print("请先创建虚拟环境:")
    print("  cd scripts")
    print("  python -m venv venv")
    sys.exit(1)

if "--protocol" in sys.argv:
    script = PROJECT_ROOT / "scripts" / "protocol-test.py"
    args = [arg for arg in sys.argv[1:] if arg != "--protocol"]
else:
    script = PROJECT_ROOT / "scripts" / "e2e-test.py"
    args = sys.argv[1:]

cmd = [str(VENV_PYTHON), str(script)] + args

try:
    result = subprocess.run(cmd, cwd=str(PROJECT_ROOT))
    sys.exit(result.returncode)
except KeyboardInterrupt:
    print("\n测试已中断")
    sys.exit(130)
except Exception as e:
    print(f"错误: {e}")
    sys.exit(1)
