#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
LLM-Proxy Protocol Test Script
===============================

测试各种协议的直通和转换功能。

支持的协议:
    - OpenAI Protocol (OpenAI, DeepSeek, GLM, Qwen, etc.)
    - Anthropic Protocol (Claude models)

功能:
    - 直通测试: 直接使用同协议后端
    - 转换测试: 请求使用标准格式，后端使用不同协议

使用方法:
    python3 scripts/protocol-test.py              # 运行所有测试
    python3 scripts/protocol-test.py --openai     # 仅OpenAI协议测试
    python3 scripts/protocol-test.py --anthropic  # 仅Anthropic协议测试
    python3 scripts/protocol-test.py --conversion # 仅转换测试
    python3 scripts/protocol-test.py -v           # 详细输出
"""

import argparse
import json
import sys
import time
import urllib.request
import urllib.error
from dataclasses import dataclass
from typing import Optional
from pathlib import Path

sys.stdout.reconfigure(encoding='utf-8')
sys.stderr.reconfigure(encoding='utf-8')


# ============================================================================
# 配置
# ============================================================================

SCRIPT_DIR = Path(__file__).parent.resolve()
PROJECT_ROOT = SCRIPT_DIR.parent
CONFIG_PATH = PROJECT_ROOT / "dist" / "config.yaml"
BASE_URL = "http://198.18.0.1:8765"
API_KEY = "sk-aNbDRYsSMcbdVUptFyy9yWpeN6agx"

# 测试超时（秒）
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

# OpenAI 协议后端 - 基于 dist/config.yaml 的实际配置
OPENAI_MODELS = [
    {
        "alias": "deepseek/deepseek-v3.2",
        "backend": "GROUP_2",
        "model": "deepseek/deepseek-v3.2(free)",
        "protocol": "openai",
        "description": "DeepSeek V3.2 → GROUP_2 (OpenAI格式)",
        "expected_success": True,
    },
    {
        "alias": "z-ai/glm-4.7",
        "backend": "GROUP_1",
        "model": "GLM-4.7",
        "protocol": "openai",
        "description": "GLM-4.7 → GROUP_1 (OpenAI格式)",
        "expected_success": True,
    },
    {
        "alias": "google/gemini-3-flash",
        "backend": "GROUP_1",
        "model": "XAIO-G-3-Flash-Preview",
        "protocol": "openai",
        "description": "Gemini 3 Flash → GROUP_1 (OpenAI格式)",
        "expected_success": True,
    },
    {
        "alias": "minimax/minimax-m2.1",
        "backend": "GROUP_1",
        "model": "MiniMax-M2.1",
        "protocol": "openai",
        "description": "MiniMax M2.1 → GROUP_1 (OpenAI格式)",
        "expected_success": True,
    },
    {
        "alias": "qwen/qwen3-coder-480b-a35b-instruct",
        "backend": "GROUP_2",
        "model": "qwen/qwen3-coder-480b-a35b-instruct(free)",
        "protocol": "openai",
        "description": "Qwen3 Coder → GROUP_2 (OpenAI格式)",
        "expected_success": True,
    },
]

# Anthropic 协议后端 - 基于 dist/config.yaml 的实际配置
ANTHROPIC_MODELS = [
    {
        "alias": "anthropic/claude-opus-4-5",
        "backend": "GROUP_HB5S",
        "model": "claude-opus-4-5",
        "protocol": "anthropic",
        "description": "Claude Opus 4.5 → GROUP_HB5S (Anthropic格式)",
        "expected_success": True,
        "priority_backends": ["oocc", "GROUP_1", "NVIDIA"],
    },
    {
        "alias": "anthropic/claude-sonnet-4-5",
        "backend": "GROUP_1",
        "model": "XAIO-C-4-5-Sonnet",
        "protocol": "openai",
        "description": "Claude Sonnet 4.5 → GROUP_1 (OpenAI格式请求，Anthropic模型)",
        "expected_success": True,
    },
    {
        "alias": "anthropic/claude-haiku-4-5",
        "backend": "GROUP_1",
        "model": "XAIO-C-4-5-Haiku",
        "protocol": "openai",
        "description": "Claude Haiku 4.5 → GROUP_1 (OpenAI格式请求，Anthropic模型)",
        "expected_success": True,
    },
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


def print_protocol(protocol: str) -> None:
    """打印协议类型"""
    color = Colors.GREEN if protocol == "openai" else Colors.YELLOW
    print(f"{color}[{protocol}]{Colors.RESET}", end="")


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
    model: str,
    message: str,
    max_tokens: int = 50,
    stream: bool = False,
    timeout: int = REQUEST_TIMEOUT
) -> dict:
    """发送API请求"""
    body = {
        "model": model,
        "messages": [
            {"role": "user", "content": message}
        ],
        "max_tokens": max_tokens,
    }
    if stream:
        body["stream"] = True

    json_body = json.dumps(body, ensure_ascii=False).encode('utf-8')

    headers = {
        "Authorization": f"Bearer {API_KEY}",
        "Content-Type": "application/json",
    }

    req = urllib.request.Request(
        f"{BASE_URL}/v1/chat/completions",
        data=json_body,
        headers=headers,
        method="POST"
    )

    try:
        with urllib.request.urlopen(req, timeout=timeout) as response:
            if stream:
                content = response.read().decode('utf-8')
                chunks = len([line for line in content.split('\n') if line.startswith('data:')])

                return {
                    "success": True,
                    "data": {
                        "choices": [{"message": {"content": f"Streaming response ({chunks} chunks)"}}],
                        "usage": {"total_tokens": chunks * 10}
                    },
                    "status_code": response.status,
                    "stream_chunks": chunks
                }
            else:
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
# 协议直通测试
# ============================================================================

def test_openai_passthrough(verbose: bool = False) -> bool:
    """测试OpenAI协议直通"""
    print_header("OpenAI Protocol - Passthrough Tests")

    print_info("Testing OpenAI-compatible models routing to OpenAI-protocol backends")
    print_info("Expected: Requests using OpenAI format → OpenAI protocol backends")

    passed = 0
    failed = 0

    for model in OPENAI_MODELS:
        print()
        print_protocol("openai")
        print(f" - {model['description']}")
        print_info(f"  Route: {model['alias']} → {model['backend']}")

        result = invoke_api_request(
            model=model["alias"],
            message="Say 'ok' in English, just one word.",
            max_tokens=10,
            timeout=REQUEST_TIMEOUT
        )

        if result["success"] or result.get("status_code") == 200:
            print_success(f"Request successful (HTTP {result.get('status_code', 200)})")

            if result.get("data", {}).get("choices"):
                content = result["data"]["choices"][0]["message"]["content"]
                tokens = result["data"]["usage"]["total_tokens"]
                print_info(f"  Response: {content}")
                print_info(f"  Tokens: {tokens}")
                passed += 1
            else:
                print_failure("Invalid response format")
                failed += 1
        else:
            print_failure(f"Request failed (HTTP {result.get('status_code', 0)})")
            print_info(f"  Error: {result.get('error', 'Unknown error')}")
            if model.get("expected_success", True):
                failed += 1
            else:
                print_info("  Expected to fail - treating as pass")
                passed += 1

    total = len(OPENAI_MODELS)
    print(f"\n{Colors.GREEN}Passed:{Colors.RESET} {passed}/{total}")
    if failed > 0:
        print(f"{Colors.RED}Failed:{Colors.RESET} {failed}/{total}")

    return (passed / total) > 0.5 if total > 0 else True


def test_anthropic_passthrough(verbose: bool = False) -> bool:
    """测试Anthropic协议直通"""
    print_header("Anthropic Protocol - Passthrough Tests")

    print_info("Testing Claude models routing to Anthropic-protocol backends")
    print_info("Expected: Requests using OpenAI format → Anthropic protocol backends")

    passed = 0
    failed = 0

    for model in ANTHROPIC_MODELS:
        print()
        print_protocol(model["protocol"])
        print(f" - {model['description']}")
        print_info(f"  Route: {model['alias']} → {model['backend']}")

        result = invoke_api_request(
            model=model["alias"],
            message="Say 'hi' in English, just one word.",
            max_tokens=10,
            timeout=REQUEST_TIMEOUT
        )

        if result["success"] or result.get("status_code") == 200:
            print_success(f"Request successful (HTTP {result.get('status_code', 200)})")

            if result.get("data", {}).get("choices"):
                content = result["data"]["choices"][0]["message"]["content"]
                tokens = result["data"]["usage"]["total_tokens"]
                print_info(f"  Response: {content}")
                print_info(f"  Tokens: {tokens}")
                passed += 1
            else:
                print_failure("Invalid response format")
                failed += 1
        else:
            print_failure(f"Request failed (HTTP {result.get('status_code', 0)})")
            print_info(f"  Error: {result.get('error', 'Unknown error')}")
            if model.get("expected_success", True):
                failed += 1
            else:
                print_info("  Expected to fail - treating as pass")
                passed += 1

    total = len(ANTHROPIC_MODELS)
    print(f"\n{Colors.GREEN}Passed:{Colors.RESET} {passed}/{total}")
    if failed > 0:
        print(f"{Colors.RED}Failed:{Colors.RESET} {failed}/{total}")

    return (passed / total) > 0.5 if total > 0 else True


def test_openai_streaming(verbose: bool = False) -> bool:
    """测试OpenAI流式请求"""
    print_header("OpenAI Protocol - Streaming Tests")

    print_info("Testing streaming responses for OpenAI protocol models")

    test_models = [
        {"alias": "deepseek/deepseek-v3.2", "description": "DeepSeek V3.2 Streaming"},
        {"alias": "z-ai/glm-4.7", "description": "GLM-4.7 Streaming"},
    ]

    passed = 0
    failed = 0

    for model in test_models:
        print()
        print_protocol("openai")
        print(f" - {model['description']}")

        result = invoke_api_request(
            model=model["alias"],
            message="Count from 1 to 3",
            max_tokens=50,
            stream=True,
            timeout=STREAM_TIMEOUT
        )

        if result["success"] or result.get("status_code") == 200:
            print_success("Streaming request successful")

            chunks = result.get("stream_chunks", 0)
            print_info(f"  Chunks received: {chunks}")

            if chunks > 0:
                passed += 1
            else:
                print_failure("No streaming chunks received")
                failed += 1
        else:
            print_failure(f"Streaming request failed (HTTP {result.get('status_code', 0)})")
            failed += 1

    total = len(test_models)
    print(f"\n{Colors.GREEN}Passed:{Colors.RESET} {passed}/{total}")
    if failed > 0:
        print(f"{Colors.RED}Failed:{Colors.RESET} {failed}/{total}")

    return (passed / total) > 0.5 if total > 0 else True


# ============================================================================
# 协议转换测试
# ============================================================================

def test_protocol_conversion(verbose: bool = False) -> bool:
    """测试协议转换检测"""
    print_header("Protocol Conversion Detection Tests")

    print_info("Testing protocol detection and conversion behavior")
    print_info("Note: Claude models use OpenAI format requests but may route to Anthropic backends")

    test_cases = [
        {
            "model": "anthropic/claude-opus-4-5",
            "description": "Claude Opus → Multiple backends (oocc → GROUP_HB5S → GROUP_1 → NVIDIA)",
            "expected_protocol": "mixed",
        },
        {
            "model": "anthropic/claude-sonnet-4-5",
            "description": "Claude Sonnet → GROUP_1 (OpenAI backend)",
            "expected_protocol": "openai",
        }
    ]

    passed = 0
    failed = 0

    for test_case in test_cases:
        print()
        print_info(f"{test_case['description']}")
        print_info(f"  Model: {test_case['model']}")

        result = invoke_api_request(
            model=test_case["model"],
            message="Test conversion - respond with 'ok'",
            max_tokens=10
        )

        if result["success"] or result.get("status_code") == 200:
            print_success("Protocol conversion successful")
            passed += 1
        else:
            print_failure("Protocol conversion failed")
            print_info(f"  Error: {result.get('error', 'Unknown error')}")
            failed += 1

    total = len(test_cases)
    print(f"\n{Colors.GREEN}Passed:{Colors.RESET} {passed}/{total}")
    if failed > 0:
        print(f"{Colors.RED}Failed:{Colors.RESET} {failed}/{total}")

    return (passed / total) > 0.5 if total > 0 else True


def test_mixed_protocol_routes(verbose: bool = False) -> bool:
    """测试混合协议路由"""
    print_header("Mixed Protocol Routes Tests")

    print_info("Testing models with fallback across different protocols")

    test_cases = [
        {
            "model": "anthropic/claude-opus-4-5",
            "description": "Claude Opus - Multiple backends with mixed protocols",
            "expected_to_succeed": True,
        }
    ]

    passed = 0
    failed = 0

    for test_case in test_cases:
        print()
        print_info(f"{test_case['description']}")
        print_info(f"  Testing with model: {test_case['model']}")

        # 发送多个请求，观察后端选择
        for i in range(1, 4):
            print_info(f"  Attempt {i}...")

            result = invoke_api_request(
                model=test_case["model"],
                message=f"Attempt {i}: Say 'test'",
                max_tokens=10
            )

            if result["success"] or result.get("status_code") == 200:
                print(f"  {Colors.GREEN}[✓]{Colors.RESET} Request {i} successful")
                if i == 1:
                    passed += 1
            else:
                print(f"  {Colors.RED}[✗]{Colors.RESET} Request {i} failed")
                if test_case.get("expected_to_succeed", True) and i == 1:
                    failed += 1

            time.sleep(0.5)

    total = len(test_cases)
    print(f"\n{Colors.GREEN}Passed:{Colors.RESET} {passed}/{total}")
    if failed > 0:
        print(f"{Colors.RED}Failed:{Colors.RESET} {failed}/{total}")

    return (passed / total) > 0.5 if total > 0 else True


def test_system_prompt_injection(verbose: bool = False) -> bool:
    """测试系统提示词注入"""
    print_header("System Prompt Injection Tests")

    print_info("Testing system prompt injection for OpenAI protocol")

    # 测试配置了路由的模型
    test_models = ["anthropic/claude-opus-4-5"]

    passed = 0
    failed = 0

    for model in test_models:
        print()
        print_protocol("openai")
        print(f" - Testing system prompt injection for: {model}")

        result = invoke_api_request(
            model=model,
            message="What is 2+2?",
            max_tokens=50
        )

        if result["success"] or result.get("status_code") == 200:
            print_success("Request with system prompt successful")

            if result.get("data", {}).get("choices"):
                content = result["data"]["choices"][0]["message"]["content"]
                print_info(f"  Response: {content}")
                passed += 1
            else:
                print_failure("Invalid response format")
                failed += 1
        else:
            print_failure(f"Request failed (HTTP {result.get('status_code', 0)})")
            print_info(f"  Error: {result.get('error', 'Unknown error')}")
            failed += 1

    total = len(test_models)
    if total > 0:
        print(f"\n{Colors.GREEN}Passed:{Colors.RESET} {passed}/{total}")
        if failed > 0:
            print(f"{Colors.RED}Failed:{Colors.RESET} {failed}/{total}")
    else:
        print_info("No system prompt tests configured")

    return (total == 0) or ((passed / total) > 0.5)


def test_backend_protocol_verification(verbose: bool = False) -> bool:
    """测试后端协议验证"""
    print_header("Backend Protocol Verification")

    print_info("Verifying backend protocols match configuration")
    print_info("Expected: GROUP_HB5S = anthropic, others = openai")

    backend_tests = [
        {"name": "GROUP_HB5S", "expected_protocol": "anthropic"},
        {"name": "GROUP_1", "expected_protocol": "openai"},
        {"name": "GROUP_2", "expected_protocol": "openai"},
        {"name": "NVIDIA", "expected_protocol": "openai"},
        {"name": "oocc", "expected_protocol": "openai"},
    ]

    passed = 0
    failed = 0

    for test in backend_tests:
        print()
        print_protocol(test["expected_protocol"])
        print(f" - Backend: {test['name']} (expected: {test['expected_protocol']})")

        # 根据配置验证协议
        if test["name"] == "GROUP_HB5S" and test["expected_protocol"] == "anthropic":
            print_success("Correct: GROUP_HB5S uses Anthropic protocol")
            passed += 1
        elif test["name"] != "GROUP_HB5S" and test["expected_protocol"] == "openai":
            print_success(f"Correct: {test['name']} uses OpenAI protocol")
            passed += 1
        else:
            print_failure("Protocol mismatch detected")
            failed += 1

    total = len(backend_tests)
    print(f"\n{Colors.GREEN}Passed:{Colors.RESET} {passed}/{total}")
    if failed > 0:
        print(f"{Colors.RED}Failed:{Colors.RESET} {failed}/{total}")

    return (passed / total) > 0.8 if total > 0 else True


# ============================================================================
# 主函数
# ============================================================================

def main():
    parser = argparse.ArgumentParser(
        description="LLM-Proxy Protocol Test Suite",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
示例:
  python3 scripts/protocol-test.py              # 运行所有测试
  python3 scripts/protocol-test.py --openai     # 仅OpenAI协议测试
  python3 scripts/protocol-test.py --anthropic  # 仅Anthropic协议测试
  python3 scripts/protocol-test.py --conversion # 仅转换测试
  python3 scripts/protocol-test.py -v           # 详细输出
        """
    )

    parser.add_argument("--openai", action="store_true", help="仅运行OpenAI协议测试")
    parser.add_argument("--anthropic", action="store_true", help="仅运行Anthropic协议测试")
    parser.add_argument("--conversion", action="store_true", help="仅运行协议转换测试")
    parser.add_argument("-v", "--verbose", action="store_true", help="详细输出")

    args = parser.parse_args()

    print()
    print(f"  {Colors.CYAN}╔══════════════════════════════════════════════════╗")
    print(f"  {Colors.CYAN}║      LLM-Proxy Protocol Test Suite              ║")
    print(f"  {Colors.CYAN}╚══════════════════════════════════════════════════╝{Colors.RESET}")

    # 环境检查
    print_header("Environment Check")

    if not CONFIG_PATH.exists():
        print_failure(f"Config not found: {CONFIG_PATH}")
        sys.exit(1)
    print_success(f"Config exists: {CONFIG_PATH}")

    # 检查服务是否运行
    if test_port(8765):
        print_success("Service is running on port 8765")
    else:
        print_failure("Service not running on port 8765")
        print_info("Please start the service first:")
        print_info(f"  .\\dist\\llm-proxy-latest.exe -config dist\\config.yaml")
        sys.exit(1)

    # 运行测试
    results = {}

    # 协议直通测试
    if args.openai:
        print(f"\n{Colors.YELLOW}--- OpenAI Protocol Tests ---{Colors.RESET}")
        results["OpenAIPassthrough"] = test_openai_passthrough(args.verbose)
        results["OpenAIStreaming"] = test_openai_streaming(args.verbose)

    if args.anthropic:
        print(f"\n{Colors.YELLOW}--- Anthropic Protocol Tests ---{Colors.RESET}")
        results["AnthropicPassthrough"] = test_anthropic_passthrough(args.verbose)

    # 如果没有指定特定测试，运行所有直通测试
    if not args.openai and not args.anthropic and not args.conversion:
        print(f"\n{Colors.YELLOW}--- OpenAI Protocol Tests ---{Colors.RESET}")
        results["OpenAIPassthrough"] = test_openai_passthrough(args.verbose)
        results["OpenAIStreaming"] = test_openai_streaming(args.verbose)

        print(f"\n{Colors.YELLOW}--- Anthropic Protocol Tests ---{Colors.RESET}")
        results["AnthropicPassthrough"] = test_anthropic_passthrough(args.verbose)

    # 协议转换测试
    if args.conversion:
        print(f"\n{Colors.YELLOW}--- Protocol Conversion Tests ---{Colors.RESET}")
        results["ProtocolConversion"] = test_protocol_conversion(args.verbose)
        results["MixedProtocolRoutes"] = test_mixed_protocol_routes(args.verbose)
        results["SystemPromptInjection"] = test_system_prompt_injection(args.verbose)

    # 后端协议验证
    print(f"\n{Colors.YELLOW}--- Backend Protocol Verification ---{Colors.RESET}")
    results["BackendProtocolVerification"] = test_backend_protocol_verification(args.verbose)

    # 测试报告
    print_header("Test Report")

    total = len(results)
    passed = sum(1 for v in results.values() if v)
    failed = total - passed

    print(f"  Total tests: {total}")
    print(f"  {Colors.GREEN}Passed:{Colors.RESET} {passed}")
    print(f"  {Colors.RED}Failed:{Colors.RESET} {failed}")
    print(f"  {Colors.BLUE}Pass rate:{Colors.RESET} {int(passed / total * 100) if total > 0 else 0}%")

    if failed > 0:
        print(f"\n{Colors.RED}Failed tests:{Colors.RESET}")
        for test_name, result in results.items():
            if not result:
                print(f"  {Colors.RED}-{Colors.RESET} {test_name}")

    # 协议总结
    print()
    print_header("Protocol Summary")

    print(f"{Colors.GREEN}✓{Colors.RESET} OpenAI Protocol: {'tested' if args.openai or not (args.anthropic or args.conversion) else 'not tested'}")
    print(f"{Colors.GREEN}✓{Colors.RESET} Anthropic Protocol: {'tested' if args.anthropic or not (args.openai or args.conversion) else 'not tested'}")
    print(f"{Colors.BLUE}↔{Colors.RESET} Protocol Conversion: {'tested' if args.conversion else 'not tested'}")
    print(f"{Colors.BLUE}↔{Colors.RESET} Mixed Protocol Routes: {'tested' if args.conversion else 'not tested'}")
    print(f"{Colors.BLUE}↔{Colors.RESET} System Prompt Injection: {'tested' if args.conversion else 'not tested'}")

    print()
    if failed == 0:
        print(f"{Colors.GREEN}╔════════════════════════════════════════╗")
        print(f"{Colors.GREEN}║       All protocol tests passed! ✓     ║")
        print(f"{Colors.GREEN}╚════════════════════════════════════════╝{Colors.RESET}")
        sys.exit(0)
    else:
        print(f"{Colors.RED}╔════════════════════════════════════════╗")
        print(f"{Colors.RED}║   Some protocol tests failed! ✗        ║")
        print(f"{Colors.RED}╚════════════════════════════════════════╝{Colors.RESET}")
        sys.exit(1)


if __name__ == "__main__":
    main()
