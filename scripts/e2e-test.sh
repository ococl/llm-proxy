#!/bin/bash
# ============================================================================
# LLM-Proxy End-to-End Test Script (Linux/macOS)
# ============================================================================
# Usage:
#   ./scripts/e2e-test.sh                    # Run all tests
#   ./scripts/e2e-test.sh --health-check     # Health check only
#   ./scripts/e2e-test.sh --normal-request   # Normal request test only
#   ./scripts/e2e-test.sh --streaming        # Streaming request test only
#   ./scripts/e2e-test.sh --verbose          # Verbose output
# ============================================================================

set -euo pipefail

# ============================================================================
# Configuration
# ============================================================================
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
CONFIG_PATH="$PROJECT_ROOT/dist/config.yaml"
BINARY_PATH="$PROJECT_ROOT/dist/llm-proxy-latest"
LOG_DIR="$PROJECT_ROOT/logs"
BASE_URL="http://localhost:8765"
API_KEY="sk-aNbDRYsSMcbdVUptFyy9yWpeN6agx"

# Timeouts (seconds)
HEALTH_TIMEOUT=5
REQUEST_TIMEOUT=30
STREAM_TIMEOUT=60

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
GRAY='\033[0;90m'
RESET='\033[0m'

# Test flags
RUN_ALL=true
RUN_HEALTH=false
RUN_NORMAL=false
RUN_STREAMING=false
VERBOSE=false

# ============================================================================
# Utility Functions
# ============================================================================

print_header() {
    echo -e "\n${CYAN}========================================"
    echo -e " $1"
    echo -e "========================================${RESET}"
}

print_success() {
    echo -e "${GREEN}[✓]${RESET} $1"
}

print_failure() {
    echo -e "${RED}[✗]${RESET} $1"
}

print_info() {
    echo -e "${BLUE}[i]${RESET} $1"
}

print_warning() {
    echo -e "${YELLOW}[!]${RESET} $1"
}

check_port() {
    local port=$1
    nc -z localhost "$port" 2>/dev/null
    return $?
}

wait_for_service() {
    local port=${1:-8765}
    local timeout=${2:-10}
    local elapsed=0

    while [ $elapsed -lt $timeout ]; do
        if check_port "$port"; then
            return 0
        fi
        sleep 0.5
        elapsed=$((elapsed + 1))
    done
    return 1
}

api_request() {
    local endpoint=$1
    local method=${2:-GET}
    local body=${3:-}
    local timeout=${4:-$REQUEST_TIMEOUT}

    local curl_opts=(
        -s
        -w "\n%{http_code}"
        -X "$method"
        -H "Authorization: Bearer $API_KEY"
        -H "Content-Type: application/json"
        --max-time "$timeout"
    )

    if [ -n "$body" ]; then
        curl_opts+=(-d "$body")
    fi

    curl "${curl_opts[@]}" "$BASE_URL$endpoint"
}

# ============================================================================
# Test Cases
# ============================================================================

test_health_check() {
    print_header "Health Check Test"

    local response
    response=$(api_request "/health" GET "" $HEALTH_TIMEOUT)
    local http_code=$(echo "$response" | tail -n1)
    local body=$(echo "$response" | head -n -1)

    if [ "$http_code" = "200" ]; then
        print_success "Health check passed"
        print_info "Response: $body"

        if echo "$body" | grep -q '"status":"healthy"'; then
            print_success "Service is healthy"
            return 0
        else
            print_failure "Service status is not healthy"
            return 1
        fi
    else
        print_failure "Health check failed (HTTP $http_code)"
        return 1
    fi
}

test_normal_request() {
    print_header "Normal Request Test"

    local models=(
        "deepseek/deepseek-v3.2:DeepSeek V3"
        "z-ai/glm-4.7:GLM-4.7"
    )

    local passed=0
    local failed=0

    for model_desc in "${models[@]}"; do
        IFS=':' read -r model description <<< "$model_desc"
        print_info "Testing model: $description ($model)"

        local body=$(cat <<EOF
{
  "model": "$model",
  "messages": [{"role": "user", "content": "你好，请用一句话回答。"}],
  "max_tokens": 50
}
EOF
)

        local response
        response=$(api_request "/v1/chat/completions" POST "$body" $REQUEST_TIMEOUT)
        local http_code=$(echo "$response" | tail -n1)
        local body_response=$(echo "$response" | head -n -1)

        if [ "$http_code" = "200" ]; then
            print_success "Request successful"
            if echo "$body_response" | grep -q '"choices"'; then
                print_info "  Response contains choices"
                ((passed++))
            else
                print_failure "  Invalid response format"
                ((failed++))
            fi
        else
            print_failure "Request failed (HTTP $http_code)"
            ((failed++))
        fi
    done

    if [ $failed -eq 0 ]; then
        print_success "All normal request tests passed ($passed/${#models[@]})"
        return 0
    else
        print_failure "Some normal request tests failed ($passed/${#models[@]})"
        return 1
    fi
}

test_streaming_request() {
    print_header "Streaming Request Test"

    local model="deepseek/deepseek-v3.2"
    print_info "Testing streaming model: $model"

    local body=$(cat <<EOF
{
  "model": "$model",
  "messages": [{"role": "user", "content": "从1数到3"}],
  "max_tokens": 100,
  "stream": true
}
EOF
)

    local response
    response=$(curl -s -w "\n%{http_code}" \
        -X POST \
        -H "Authorization: Bearer $API_KEY" \
        -H "Content-Type: application/json" \
        -d "$body" \
        --max-time $STREAM_TIMEOUT \
        "$BASE_URL/v1/chat/completions")

    local http_code=$(echo "$response" | tail -n1)
    local body_response=$(echo "$response" | head -n -1)

    if [ "$http_code" = "200" ]; then
        local chunks=$(echo "$body_response" | grep -c "^data:" || true)
        print_success "Streaming request successful"
        print_info "  Received $chunks data chunks"
        return 0
    else
        print_failure "Streaming request failed (HTTP $http_code)"
        return 1
    fi
}

test_error_handling() {
    print_header "Error Handling Test"

    local body=$(cat <<EOF
{
  "model": "invalid/model-not-exist",
  "messages": [{"role": "user", "content": "Test"}]
}
EOF
)

    local response
    response=$(api_request "/v1/chat/completions" POST "$body" $REQUEST_TIMEOUT)
    local http_code=$(echo "$response" | tail -n1)

    if [ "$http_code" != "200" ]; then
        print_success "Invalid model request correctly returned error (HTTP $http_code)"
        return 0
    else
        print_failure "Invalid model request should return error"
        return 1
    fi
}

test_logging() {
    print_header "Logging Verification"

    local log_files=(
        "$LOG_DIR/general.log"
        "$LOG_DIR/requests/request.log"
    )

    local found_logs=false
    for log_file in "${log_files[@]}"; do
        if [ -f "$log_file" ]; then
            local size=$(stat -f%z "$log_file" 2>/dev/null || stat -c%s "$log_file" 2>/dev/null || echo "0")
            print_info "Log file exists: $log_file ($size bytes)"
            found_logs=true

            if [ "$size" -gt 0 ]; then
                print_info "Log preview (last 5 lines):"
                tail -n 5 "$log_file" | while IFS= read -r line; do
                    echo -e "  ${GRAY}$line${RESET}"
                done
            fi
        fi
    done

    if [ "$found_logs" = true ]; then
        print_success "Logging system is working"
        return 0
    else
        print_warning "No log files found"
        return 1
    fi
}

# ============================================================================
# Main Function
# ============================================================================

main() {
    # Parse arguments
    while [[ $# -gt 0 ]]; do
        case $1 in
            --health-check)
                RUN_ALL=false
                RUN_HEALTH=true
                shift
                ;;
            --normal-request)
                RUN_ALL=false
                RUN_NORMAL=true
                shift
                ;;
            --streaming)
                RUN_ALL=false
                RUN_STREAMING=true
                shift
                ;;
            --verbose)
                VERBOSE=true
                shift
                ;;
            *)
                echo "Unknown option: $1"
                exit 1
                ;;
        esac
    done

    echo -e "\n${CYAN}"
    echo "  ╔══════════════════════════════════════════════════╗"
    echo "  ║          LLM-Proxy E2E Test Suite                ║"
    echo "  ╚══════════════════════════════════════════════════╝"
    echo -e "${RESET}"

    # Environment check
    print_header "Environment Check"

    if [ ! -f "$BINARY_PATH" ]; then
        print_failure "Binary not found: $BINARY_PATH"
        print_info "Please build first: cd src && go build -o ../dist/llm-proxy-latest ."
        exit 1
    fi
    print_success "Binary exists: $BINARY_PATH"

    if [ ! -f "$CONFIG_PATH" ]; then
        print_failure "Config not found: $CONFIG_PATH"
        exit 1
    fi
    print_success "Config exists: $CONFIG_PATH"

    # Check if service is running
    local service_running=false
    if check_port 8765; then
        print_warning "Port 8765 is already in use, assuming service is running"
        service_running=true
    fi

    # Start service if not running
    local pid=""
    if [ "$service_running" = false ]; then
        print_info "Starting service..."
        "$BINARY_PATH" -config "$CONFIG_PATH" &
        pid=$!

        if ! wait_for_service 8765 10; then
            print_failure "Service failed to start"
            exit 1
        fi
        print_success "Service started (PID: $pid)"
    fi

    # Create log directory
    mkdir -p "$LOG_DIR"

    # Run tests
    declare -A results

    if [ "$RUN_ALL" = true ] || [ "$RUN_HEALTH" = true ]; then
        test_health_check && results[HealthCheck]=1 || results[HealthCheck]=0
    fi

    if [ "$RUN_ALL" = true ] || [ "$RUN_NORMAL" = true ]; then
        test_normal_request && results[NormalRequest]=1 || results[NormalRequest]=0
    fi

    if [ "$RUN_ALL" = true ] || [ "$RUN_STREAMING" = true ]; then
        test_streaming_request && results[StreamingRequest]=1 || results[StreamingRequest]=0
    fi

    test_error_handling && results[ErrorHandling]=1 || results[ErrorHandling]=0
    test_logging && results[Logging]=1 || results[Logging]=0

    # Stop service if we started it
    if [ -n "$pid" ] && [ "$service_running" = false ]; then
        print_header "Stopping Service"
        kill "$pid" 2>/dev/null || true
        wait "$pid" 2>/dev/null || true
        print_success "Service stopped"
    fi

    # Test report
    print_header "Test Report"

    local total=${#results[@]}
    local passed=0
    local failed=0

    for result in "${results[@]}"; do
        if [ "$result" -eq 1 ]; then
            ((passed++))
        else
            ((failed++))
        fi
    done

    echo "  Total tests: $total"
    echo -e "  ${GREEN}Passed: $passed${RESET}"
    echo -e "  ${RED}Failed: $failed${RESET}"
    echo -e "  ${BLUE}Pass rate: $(( passed * 100 / total ))%${RESET}"

    if [ $failed -gt 0 ]; then
        echo -e "\n${RED}Failed tests:${RESET}"
        for test in "${!results[@]}"; do
            if [ "${results[$test]}" -eq 0 ]; then
                echo -e "  ${RED}- $test${RESET}"
            fi
        done
    fi

    echo ""
    if [ $failed -eq 0 ]; then
        echo -e "${GREEN}╔════════════════════════════════════════╗"
        echo -e "║          All tests passed! ✓           ║"
        echo -e "╚════════════════════════════════════════╝${RESET}"
        exit 0
    else
        echo -e "${RED}╔════════════════════════════════════════╗"
        echo -e "║       Some tests failed! ✗             ║"
        echo -e "╚════════════════════════════════════════╝${RESET}"
        exit 1
    fi
}

main "$@"
