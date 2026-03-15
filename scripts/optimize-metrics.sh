#!/usr/bin/env bash
# Captures optimization metrics as JSON for before/after comparison.
# Usage: ./scripts/optimize-metrics.sh [output-file]
# Output: JSON with test results, lint counts, benchmarks, code stats, binary size.
set -euo pipefail

OUT="${1:-/dev/stdout}"
REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$REPO_ROOT"

# Ensure GOPATH/bin is in PATH for golangci-lint
export PATH="$(go env GOPATH)/bin:$PATH"

tmp_dir=$(mktemp -d)
trap 'rm -rf "$tmp_dir"' EXIT

# --- Tests ---
tests_pass=true
test_count=0
if go test -count=1 -timeout 120s ./... > "$tmp_dir/test.out" 2>&1; then
    tests_pass=true
else
    tests_pass=false
fi
test_count=$(grep -c '^ok\|^FAIL' "$tmp_dir/test.out" 2>/dev/null || echo 0)

# --- Lint (strict config for optimization) ---
lint_issues=0
if [ -f .golangci-optimize.yml ]; then
    lint_cfg=".golangci-optimize.yml"
else
    lint_cfg=".golangci.yml"
fi
if command -v golangci-lint > /dev/null; then
    golangci-lint run --config "$lint_cfg" --out-format json ./... > "$tmp_dir/lint.json" 2>/dev/null || true
    lint_issues=$(python3 -c "
import json, sys
try:
    d = json.load(open('$tmp_dir/lint.json'))
    print(len(d.get('Issues', []) if d.get('Issues') else []))
except: print(0)
" 2>/dev/null || echo 0)
fi

# --- Binary size ---
binary_size=0
if go build -ldflags="-s -w" -o "$tmp_dir/grove-bin" ./cmd/grove 2>/dev/null; then
    binary_size=$(stat -f%z "$tmp_dir/grove-bin" 2>/dev/null || stat -c%s "$tmp_dir/grove-bin" 2>/dev/null || echo 0)
fi

# --- Code stats (non-test files) ---
total_lines=$(find . -name '*.go' -not -path './vendor/*' -not -name '*_test.go' | xargs wc -l 2>/dev/null | tail -1 | awk '{print $1}')
total_files=$(find . -name '*.go' -not -path './vendor/*' -not -name '*_test.go' | wc -l | tr -d ' ')
total_funcs=$(grep -r '^func ' --include='*.go' --exclude='*_test.go' . 2>/dev/null | wc -l | tr -d ' ')

# --- Benchmarks ---
bench_json="[]"
if go test -bench=. -benchmem -count=1 -run='^$' -timeout 120s ./internal/tui/ > "$tmp_dir/bench.out" 2>&1; then
    bench_json=$(python3 -c "
import re, json, sys
results = []
for line in open('$tmp_dir/bench.out'):
    m = re.match(r'(Benchmark\w+)-\d+\s+\d+\s+([\d.]+)\s+ns/op\s+(\d+)\s+B/op\s+(\d+)\s+allocs/op', line)
    if m:
        results.append({
            'name': m.group(1),
            'ns_op': float(m.group(2)),
            'bytes_op': int(m.group(3)),
            'allocs_op': int(m.group(4))
        })
print(json.dumps(results))
" 2>/dev/null || echo "[]")
fi

# --- Assemble JSON ---
cat > "$OUT" <<ENDJSON
{
  "timestamp": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
  "tests_pass": $tests_pass,
  "test_packages": $test_count,
  "lint_issues": $lint_issues,
  "binary_size_bytes": $binary_size,
  "code": {
    "total_lines": $total_lines,
    "total_files": $total_files,
    "total_functions": $total_funcs
  },
  "benchmarks": $bench_json
}
ENDJSON
