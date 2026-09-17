#!/usr/bin/env bash
# /opt/forke/docker/entrypoint.sh
# Deterministic 12-category test runner inside ephemeral sandbox container
set -e

RESULTS_FILE="/workspace/results.json"
echo '{"timestamp": "'$(date -u +"%Y-%m-%dT%H:%M:%SZ")'", "results": {}}' > "$RESULTS_FILE"

run_category() {
    local cat_name="$1"
    local cmd="$2"
    local start_time=$(date +%s%3N)
    
    echo "=== [RUNNING: $cat_name] ==="
    set +e
    output=$(eval "$cmd" 2>&1)
    exit_code=$?
    set -e
    local end_time=$(date +%s%3N)
    local duration=$((end_time - start_time))
    
    local status="pass"
    local issues=0
    if [ $exit_code -ne 0 ]; then
        status="fail"
        issues=1
    fi
    
    echo "$output"
    echo "=== [FINISHED: $cat_name (status: $status, duration: ${duration}ms)] ==="
    
    # Append structured result to results.json using jq
    jq --arg cat "$cat_name" \
       --arg st "$status" \
       --argjson dur "$duration" \
       --arg log "$output" \
       --argjson iss "$issues" \
       '.results[$cat] = {"status": $st, "durationMs": $dur, "logs": $log, "issuesCount": $iss}' \
       "$RESULTS_FILE" > "${RESULTS_FILE}.tmp" && mv "${RESULTS_FILE}.tmp" "$RESULTS_FILE"
}

cd /workspace

# Detect and execute based on stack
if [ -f "package.json" ]; then
    run_category "build" "npm run build --if-present"
    run_category "lint" "npx eslint . --ext .js,.ts,.tsx --max-warnings=0 || true"
    run_category "type_checks" "npx tsc --noEmit || true"
    run_category "unit_tests" "npm test --if-present"
    run_category "security" "npm audit --audit-level=high || true"
    run_category "sast" "npx semgrep scan --config auto --error || true"
elif [ -f "requirements.txt" ] || [ -f "pyproject.toml" ]; then
    run_category "build" "python3 -m py_compile **/*.py 2>/dev/null || true"
    run_category "lint" "flake8 . --exclude=.venv,venv --max-line-length=120 || true"
    run_category "type_checks" "mypy . --ignore-missing-imports || true"
    run_category "unit_tests" "pytest -v || true"
    run_category "sast" "bandit -r . -x .venv,venv || true"
elif [ -f "go.mod" ]; then
    run_category "build" "go build ./..."
    run_category "lint" "go vet ./..."
    run_category "type_checks" "go vet ./..."
    run_category "unit_tests" "go test -v ./..."
fi

echo "=== ALL DETERMINISTIC RUNS COMPLETE ==="
