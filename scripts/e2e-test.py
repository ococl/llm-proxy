#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
LLM-Proxy End-to-End Test Script
==================================

LLM-Proxy的端到端测试套件。

使用方法:
    python3 scripts/e2e-test.py                    # 运行所有核心测试
    python3 scripts/e2e-test.py --health-check     # 仅健康检查
    python3 scripts/e2e-test.py --normal-request   # 仅正常请求测试
    python3 scripts/e2e-test.py --streaming        # 仅流式请求测试
    python3 scripts/e2e-test.py --protocol         # 仅协议测试
    python3 scripts/e2e-test.py --all              # 运行所有测试
    python3 scripts/e2e-test.py -v                 # 详细输出
"""

import argparse
import json
import subprocess
import sys
import time
import urllib.request
import urllib.error
from pathlib import Path
from typing import Optional, Dict, Any

sys.stdout.reconfigure(encoding='utf-8')
sys.stderr.reconfigure(encoding='utf-8')


# ============================================================================
# 配置
# ============================================================================

SCRIPT_DIR = Path(__file__).parent.resolve()
PROJECT_ROOT = SCRIPT_DIR.parent
CONFIG_PATH = PROJECT_ROOT / "dist" / "config.yaml"
BINARY_PATH = PROJECT_ROOT / "dist" / "llm-proxy-latest.exe"
PROTOCOL_TEST_PATH = SCRIPT_DIR / "protocol-test.py"
LOG_DIR = PROJECT_ROOT / "logs"
BASE_URL = "http://198.18.0.1:8765"
API_KEY = "sk-aNbDRYsSMcbdVUptFyy9yWpeN6agx"

# 测试超时（秒）
HEALTH_TIMEOUT = 5
REQUEST_TIMEOUT = 30
STREAM_TIMEOUT = 60

# 颜色代码
class Colors:
    CYAN = '\033[0;36m'
    GREEN = '\033[0;32m'
    RED = '\033[0;31m'
    YELLOW = '\033[1;33m'
    BLUE = '\033[0;34m'
    RESET = '\033[0m'


# ============================================================================
# 测试数据
# ============================================================================

# 正常请求测试模型
NORMAL_TEST_MODELS = [
    {"model": "deepseek/deepseek-v3.2", "description": "DeepSeek V3"},
    {"model": "z-ai/glm-4.7", "description": "GLM-4.7"},
]

# 流式请求测试模型
STREAMING_TEST_MODELS = [
    {"model": "deepseek/deepseek-v3.2", "description": "DeepSeek V3 (流式)"},
]


# ============================================================================
# 工具函数
# ============================================================================

def print_header(title: str) -> None:
    """打印标题"""
    print(f"\n{Colors.CYAN}========================================{Colors.RESET}")
    print(f" {title}")
    print(f"{Colors.CYAN}========================================{Colors.RESET}")


def print_success(message: str) -> None:
    """打印成功消息"""
    print(f"{Colors.GREEN}[✓]{Colors.RESET} {message}")


def print_failure(message: str) -> None:
    """打印失败消息"""
    print(f"{Colors.RED}[✗]{Colors.RESET} {message}")


def print_info(message: str) -> None:
    """打印信息消息"""
    print(f"{Colors.BLUE}[i]{Colors.RESET} {message}")


def print_warning(message: str) -> None:
    """打印警告消息"""
    print(f"{Colors.YELLOW}[!]{Colors.RESET} {message}")


def test_port(port: int) -> bool:
    """测试端口是否开放"""
    import socket
    sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    sock.settimeout(1)
    try:
        result = sock.connect_ex(('localhost', port))
        return result == 0
    except Exception:
        return False
    finally:
        sock.close()


def wait_for_service(port: int, timeout: int = 10000, interval: int = 500) -> bool:
    """等待服务启动"""
    import time as time_module
    start_time = time_module.time()
    timeout_seconds = timeout / 1000

    while (time_module.time() - start_time) < timeout_seconds:
        if test_port(port):
            return True
        time_module.sleep(interval / 1000)

    return False


def invoke_api_request(
    endpoint: str,
    method: str = "GET",
    body: Optional[dict] = None,
    timeout: int = REQUEST_TIMEOUT,
    stream: bool = False
) -> dict:
    """发送API请求"""
    headers = {
        "Authorization": f"Bearer {API_KEY}",
        "Content-Type": "application/json",
    }

    url = f"{BASE_URL}{endpoint}"

    try:
        if body:
            json_body = json.dumps(body, ensure_ascii=False).encode('utf-8')
            req = urllib.request.Request(url, data=json_body, headers=headers, method=method)
        else:
            req = urllib.request.Request(url, headers=headers, method=method)

        if stream:
            with urllib.request.urlopen(req, timeout=timeout) as response:
                content = response.read().decode('utf-8')
                return {
                    "success": True,
                    "data": {"content": content},
                    "status_code": response.status,
                }
        else:
            with urllib.request.urlopen(req, timeout=timeout) as response:
                data = json.loads(response.read().decode('utf-8'))
                return {
                    "success": True,
                    "data": data,
                    "status_code": response.status,
                }

    except urllib.error.HTTPError as e:
        error_body = e.read().decode('utf-8') if e.fp else str(e)
        return {
            "success": False,
            "error": error_body,
            "status_code": e.code,
        }
    except urllib.error.URLError as e:
        return {
            "success": False,
            "error": str(e.reason),
            "status_code": 0,
        }
    except Exception as e:
        return {
            "success": False,
            "error": str(e),
            "status_code": 0,
        }


# ============================================================================
# 测试用例
# ============================================================================

def test_health_check() -> bool:
    """健康检查测试"""
    print_header("健康检查测试")

    result = invoke_api_request("/health", timeout=HEALTH_TIMEOUT)

    if result["success"]:
        print_success("服务健康检查通过")
        print_info(f"状态: {result['data'].get('status', 'unknown')}")
        print_info(f"后端数量: {result['data'].get('backends', 'unknown')}")
        print_info(f"模型数量: {result['data'].get('models', 'unknown')}")

        if result['data'].get('status') == 'healthy':
            print_success("服务运行正常")
            return True
        else:
            print_failure("服务状态异常")
            return False
    else:
        print_failure(f"健康检查失败: {result.get('error', 'Unknown error')}")
        return False


def test_normal_request() -> bool:
    """正常请求测试"""
    print_header("正常请求测试")

    passed = 0
    failed = 0

    for test_model in NORMAL_TEST_MODELS:
        print_info(f"测试模型: {test_model['description']} ({test_model['model']})")

        body = {
            "model": test_model["model"],
            "messages": [
                {"role": "user", "content": "你好，请用一句话回答。"}
            ],
            "max_tokens": 50
        }

        result = invoke_api_request(
            endpoint="/v1/chat/completions",
            method="POST",
            body=body,
            timeout=REQUEST_TIMEOUT
        )

        if result["success"] and result.get("data", {}).get("choices"):
            print_success("请求成功")
            content = result["data"]["choices"][0]["message"]["content"]
            tokens = result["data"]["usage"]["total_tokens"]
            print_info(f"  回复长度: {len(content)} 字符")
            print_info(f"  Token使用: {tokens}")
            passed += 1
        else:
            print_failure(f"请求失败: {result.get('error', 'Unknown error')}")
            failed += 1

    total = len(NORMAL_TEST_MODELS)
    if passed > 0 and failed == 0:
        print_success(f"正常请求测试全部通过 ({passed}/{total})")
        return True
    else:
        print_failure(f"正常请求测试部分失败 ({passed}/{total})")
        return False


def test_streaming_request() -> bool:
    """流式请求测试"""
    print_header("流式请求测试")

    passed = 0
    failed = 0

    for test_model in STREAMING_TEST_MODELS:
        print_info(f"测试流式模型: {test_model['description']}")

        body = {
            "model": test_model["model"],
            "messages": [
                {"role": "user", "content": "从1数到3"}
            ],
            "max_tokens": 100,
            "stream": True
        }

        result = invoke_api_request(
            endpoint="/v1/chat/completions",
            method="POST",
            body=body,
            timeout=STREAM_TIMEOUT,
            stream=True
        )

        if result["success"] and result.get("status_code") == 200:
            content = result["data"]["content"]
            # 计算流式块数量
            chunks = len([line for line in content.split('\n') if line.startswith('data:')])
            print_success("流式请求成功")
            print_info(f"  收到 {chunks} 个数据块")
            passed += 1
        else:
            print_failure(f"流式请求失败 (HTTP {result.get('status_code', 0)})")
            failed += 1

    total = len(STREAMING_TEST_MODELS)
    if passed > 0 and failed == 0:
        print_success(f"流式请求测试全部通过 ({passed}/{total})")
        return True
    else:
        print_failure(f"流式请求测试部分失败 ({passed}/{total})")
        return False


def test_error_handling() -> bool:
    """错误处理测试"""
    print_header("错误处理测试")

    # 测试无效模型
    body = {
        "model": "invalid/model-not-exist",
        "messages": [
            {"role": "user", "content": "Test"}
        ]
    }

    result = invoke_api_request(
        endpoint="/v1/chat/completions",
        method="POST",
        body=body,
        timeout=REQUEST_TIMEOUT
    )

    if not result["success"]:
        print_success(f"无效模型请求正确返回错误 (HTTP {result.get('status_code', 0)})")
        return True
    else:
        print_failure("无效模型请求应该返回错误")
        return False


def test_logging() -> bool:
    """日志验证"""
    print_header("日志验证")

    log_files = [
        LOG_DIR / "general.log",
        LOG_DIR / "requests" / "request.log"
    ]

    found_logs = False
    for log_file in log_files:
        if log_file.exists():
            size = log_file.stat().st_size
            print_info(f"日志文件存在: {log_file} ({size} bytes)")
            found_logs = True

            # 显示最后几行
            if size > 0:
                print_info("日志预览 (最后5行):")
                try:
                    with open(log_file, 'r', encoding='utf-8') as f:
                        lines = f.readlines()[-5:]
                        for line in lines:
                            print(f"  {line.rstrip()}")
                except Exception:
                    pass

    if found_logs:
        print_success("日志系统正常")
        return True
    else:
        print_warning("未找到日志文件")
        return False


# ============================================================================
# 协议测试集成
# ============================================================================

def test_protocol_tests(verbose: bool = False) -> bool:
    """协议测试"""
    print_header("协议测试")

    # 检查协议测试脚本是否存在
    if not PROTOCOL_TEST_PATH.exists():
        print_failure(f"协议测试脚本不存在: {PROTOCOL_TEST_PATH}")
        return False
    print_success(f"协议测试脚本存在: {PROTOCOL_TEST_PATH}")

    # 运行协议测试
    print_info("启动协议测试套件...")

    try:
        result = subprocess.run(
            [sys.executable, str(PROTOCOL_TEST_PATH)],
            capture_output=True,
            text=True,
            timeout=120
        )

        # 打印输出
        if result.stdout:
            print(result.stdout)

        if result.stderr:
            print(result.stderr, file=sys.stderr)

        if result.returncode == 0:
            print_success("协议测试通过")
            return True
        else:
            print_failure(f"协议测试失败 (退出码: {result.returncode})")
            return False

    except subprocess.TimeoutExpired:
        print_failure("协议测试超时")
        return False
    except Exception as e:
        print_failure(f"协议测试执行异常: {e}")
        return False


def test_openai_protocol_passthrough() -> bool:
    """OpenAI协议透传测试"""
    print_header("OpenAI 协议透传测试")

    print_info("测试 OpenAI 兼容模型的协议透传功能")

    test_models = [
        {"model": "deepseek/deepseek-v3.2", "backend": "GROUP_2"},
        {"model": "z-ai/glm-4.7", "backend": "GROUP_1"},
        {"model": "google/gemini-3-flash", "backend": "GROUP_1"},
    ]

    passed = 0
    failed = 0

    for test_model in test_models:
        print_info(f"测试模型: {test_model['model']} → {test_model['backend']}")

        body = {
            "model": test_model["model"],
            "messages": [
                {"role": "user", "content": "Say 'ok' in English."}
            ],
            "max_tokens": 10
        }

        result = invoke_api_request(
            endpoint="/v1/chat/completions",
            method="POST",
            body=body,
            timeout=REQUEST_TIMEOUT
        )

        if result["success"] and result.get("data", {}).get("choices"):
            print_success("请求成功")
            passed += 1
        else:
            print_failure("请求失败")
            failed += 1

    total = len(test_models)
    if passed > 0 and failed == 0:
        print_success(f"OpenAI 协议透传测试通过 ({passed}/{total})")
        return True
    else:
        print_failure(f"OpenAI 协议透传测试部分失败 ({passed}/{total})")
        return False


def test_anthropic_protocol_passthrough() -> bool:
    """Anthropic协议透传测试"""
    print_header("Anthropic 协议透传测试")

    print_info("测试 Claude 模型的协议透传功能")

    test_models = [
        {"model": "anthropic/claude-opus-4-5", "backend": "GROUP_HB5S (Anthropic)"},
        {"model": "anthropic/claude-sonnet-4-5", "backend": "GROUP_1 (OpenAI)"},
    ]

    passed = 0
    failed = 0

    for test_model in test_models:
        print_info(f"测试模型: {test_model['model']} → {test_model['backend']}")

        body = {
            "model": test_model["model"],
            "messages": [
                {"role": "user", "content": "Say 'hi' in English."}
            ],
            "max_tokens": 10
        }

        result = invoke_api_request(
            endpoint="/v1/chat/completions",
            method="POST",
            body=body,
            timeout=REQUEST_TIMEOUT
        )

        if result["success"] and result.get("data", {}).get("choices"):
            print_success("请求成功")
            passed += 1
        else:
            print_failure("请求失败")
            failed += 1

    total = len(test_models)
    if passed > 0 and failed == 0:
        print_success(f"Anthropic 协议透传测试通过 ({passed}/{total})")
        return True
    else:
        print_failure(f"Anthropic 协议透传测试部分失败 ({passed}/{total})")
        return False


# ============================================================================
# 主函数
# ============================================================================

def main():
    parser = argparse.ArgumentParser(
        description="LLM-Proxy End-to-End Test Suite",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
示例:
  python3 scripts/e2e-test.py              # 运行所有核心测试
  python3 scripts/e2e-test.py --all        # 运行所有测试
  python3 scripts/e2e-test.py --health     # 仅健康检查
  python3 scripts/e2e-test.py --protocol   # 仅协议测试
  python3 scripts/e2e-test.py -v           # 详细输出
        """
    )

    parser.add_argument("--all", action="store_true", help="运行所有测试")
    parser.add_argument("--health", action="store_true", help="仅健康检查")
    parser.add_argument("--normal", action="store_true", help="仅正常请求测试")
    parser.add_argument("--streaming", action="store_true", help="仅流式请求测试")
    parser.add_argument("--protocol", action="store_true", help="仅协议测试")
    parser.add_argument("--openai", action="store_true", help="仅OpenAI协议透传测试")
    parser.add_argument("--anthropic", action="store_true", help="仅Anthropic协议透传测试")
    parser.add_argument("-v", "--verbose", action="store_true", help="详细输出")

    args = parser.parse_args()

    print()
    print(f"  {Colors.CYAN}╔══════════════════════════════════════════════════╗")
    print(f"  {Colors.CYAN}║          LLM-Proxy 端到端测试套件                 ║")
    print(f"  {Colors.CYAN}╚══════════════════════════════════════════════════╝{Colors.RESET}")

    # 检查前置条件
    print_header("环境检查")

    if not BINARY_PATH.exists():
        print_failure(f"未找到二进制文件: {BINARY_PATH}")
        print_info("请先运行: cd src && go build -o ../dist/llm-proxy-latest.exe .")
        sys.exit(1)
    print_success(f"二进制文件存在: {BINARY_PATH}")

    if not CONFIG_PATH.exists():
        print_failure(f"未找到配置文件: {CONFIG_PATH}")
        sys.exit(1)
    print_success(f"配置文件存在: {CONFIG_PATH}")

    # 检查服务是否已运行
    if test_port(8765):
        print_warning("检测到端口 8765 已被占用，假设服务已运行")
        service_running = True
    else:
        service_running = False

    # 启动服务（如果未运行）
    process = None
    if not service_running:
        print_info("启动服务...")
        try:
            process = subprocess.Popen(
                [str(BINARY_PATH), "-config", str(CONFIG_PATH)],
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE
            )

            if not wait_for_service(8765, timeout=10000):
                print_failure("服务启动失败")
                sys.exit(1)
            print_success(f"服务已启动 (PID: {process.pid})")
        except Exception as e:
            print_failure(f"服务启动异常: {e}")
            sys.exit(1)

    # 创建日志目录
    try:
        LOG_DIR.mkdir(parents=True, exist_ok=True)
        (LOG_DIR / "requests").mkdir(parents=True, exist_ok=True)
        (LOG_DIR / "errors").mkdir(parents=True, exist_ok=True)
    except Exception:
        pass

    # 执行测试
    results = {}

    if args.all or args.health:
        results["HealthCheck"] = test_health_check()

    if args.all or args.normal:
        results["NormalRequest"] = test_normal_request()

    if args.all or args.streaming:
        results["StreamingRequest"] = test_streaming_request()

    if args.all or args.protocol:
        results["ProtocolTests"] = test_protocol_tests(args.verbose)

    if args.all or args.openai:
        results["OpenAIProtocolPassthrough"] = test_openai_protocol_passthrough()

    if args.all or args.anthropic:
        results["AnthropicProtocolPassthrough"] = test_anthropic_protocol_passthrough()

    if args.all:
        results["ErrorHandling"] = test_error_handling()
        results["Logging"] = test_logging()

    # 如果没有指定任何测试，运行核心测试
    if not any([args.all, args.health, args.normal, args.streaming, args.protocol, args.openai, args.anthropic]):
        results["HealthCheck"] = test_health_check()
        results["NormalRequest"] = test_normal_request()
        results["StreamingRequest"] = test_streaming_request()

    # 停止服务（如果由我们启动）
    if process:
        print_header("停止服务")
        process.terminate()
        try:
            process.wait(timeout=5)
        except subprocess.TimeoutExpired:
            process.kill()
        print_success("服务已停止")

    # 输出测试报告
    print_header("测试报告")

    total = len(results)
    passed = sum(1 for v in results.values() if v)
    failed = total - passed

    print(f"  总测试数: {total}")
    print(f"  {Colors.GREEN}通过:{Colors.RESET} {passed}")
    print(f"  {Colors.RED}失败:{Colors.RESET} {failed}")
    print(f"  {Colors.BLUE}通过率:{Colors.RESET} {int(passed / total * 100) if total > 0 else 0}%")

    if failed > 0:
        print(f"\n{Colors.RED}失败的测试:{Colors.RESET}")
        for test_name, result in results.items():
            if not result:
                print(f"  {Colors.RED}-{Colors.RESET} {test_name}")

    # 测试类型统计
    print()
    print(f"  {Colors.CYAN}测试类型统计:{Colors.RESET}")
    print(f"    - 核心功能测试: {'运行' if args.all or not any([args.health, args.normal, args.streaming, args.protocol, args.openai, args.anthropic]) else '跳过'}")
    print(f"    - 协议测试: {'运行' if args.all or args.protocol else '跳过'}")
    print(f"    - OpenAI 协议: {'运行' if args.all or args.openai else '跳过'}")
    print(f"    - Anthropic 协议: {'运行' if args.all or args.anthropic else '跳过'}")

    print()
    if failed == 0:
        print(f"{Colors.GREEN}╔════════════════════════════════════════╗")
        print(f"{Colors.GREEN}║          所有测试通过! ✓               ║")
        print(f"{Colors.GREEN}╚════════════════════════════════════════╝{Colors.RESET}")
        sys.exit(0)
    else:
        print(f"{Colors.RED}╔════════════════════════════════════════╗")
        print(f"{Colors.RED}║          部分测试失败! ✗               ║")
        print(f"{Colors.RED}╚════════════════════════════════════════╝{Colors.RESET}")
        sys.exit(1)


if __name__ == "__main__":
    main()
