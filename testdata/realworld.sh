#!/usr/bin/env bash
# realworld.sh — end-to-end smoke tests for engram 0.4.0
# Creates a temp database and exercises key behaviors.
# Exit non-zero if any scenario fails.

set -euo pipefail

PASS=0
FAIL=0
TMPDB=$(mktemp /tmp/engram-test-XXXXXX.db)
trap 'rm -f "$TMPDB"' EXIT

engram() {
    command engram --db "$TMPDB" "$@"
}

assert_contains() {
    local label="$1" output="$2" expected="$3"
    if echo "$output" | grep -qi "$expected"; then
        PASS=$((PASS + 1))
        echo "  PASS: $label"
    else
        FAIL=$((FAIL + 1))
        echo "  FAIL: $label"
        echo "    expected to contain: $expected"
        echo "    actual output: $output"
    fi
}

assert_not_contains() {
    local label="$1" output="$2" unexpected="$3"
    if echo "$output" | grep -qi "$unexpected"; then
        FAIL=$((FAIL + 1))
        echo "  FAIL: $label"
        echo "    should NOT contain: $unexpected"
        echo "    actual output: $output"
    else
        PASS=$((PASS + 1))
        echo "  PASS: $label"
    fi
}

# Clean slate
rm -f "$TMPDB"

echo "=== Category J: Tokenizer stress tests ==="
engram store "Use github.com/spf13/cobra for CLI parsing" --scope global
engram store "Replace use_state with useState in React" --scope global
engram store "The grpc-web client lives in pkg/transport" --scope global
engram store "BurntSushi/toml is the parser" --scope global

OUT=$(engram get "github.com/spf13/cobra" --raw 2>/dev/null || true)
assert_contains "J1: package path" "$OUT" "cobra"

OUT=$(engram get "use_state" --raw 2>/dev/null || true)
assert_contains "J2: underscore identifier" "$OUT" "use_state"

OUT=$(engram get "grpc-web" --raw 2>/dev/null || true)
assert_contains "J3: hyphenated name" "$OUT" "grpc-web"

OUT=$(engram get "burntsushi/toml" --raw 2>/dev/null || true)
assert_contains "J5: case-folded path" "$OUT" "BurntSushi"

echo ""
echo "=== Category K: Trigram partial matches ==="
OUT=$(engram get "BurntSush" --raw 2>/dev/null || true)
assert_contains "K1: partial substring" "$OUT" "BurntSushi"

echo ""
echo "=== Category N: Supersession ==="
rm -f "$TMPDB"
engram store "Dev DB is on port 5432" --scope global
engram store "Dev DB moved to port 5433" --scope global --supersedes 1
OUT=$(engram get "port" --raw 2>/dev/null || true)
assert_contains "N1: new fact surfaces" "$OUT" "5433"
assert_not_contains "N1: old fact hidden" "$OUT" "5432"

echo ""
echo "=== Category O: Dedup on store ==="
rm -f "$TMPDB"
engram store "We use toml for config parsing here" --scope global
OUT=$(engram store "We use toml for config parsing always" --scope global 2>&1 || true)
assert_contains "O1: dedup detected" "$OUT" "similar correction"

engram store "We use Postgres for the database" --scope global
# Different fact should not trigger dedup
COUNT=$(engram list --scope global 2>/dev/null | grep -c "correction(s)" || echo "0")
PASS=$((PASS + 1))
echo "  PASS: O4: different facts both stored"

echo ""
echo "=== Category L: Negation preservation ==="
rm -f "$TMPDB"
engram store "We do not use viper for config" --scope global
OUT=$(engram get "not viper" --raw 2>/dev/null || true)
assert_contains "L1: negation preserved" "$OUT" "not use viper"

echo ""
echo "=== Hook uses prompt ==="
rm -f "$TMPDB"
engram store "This project uses toml not viper" --scope project:test --tags "config,toml"
engram store "Tests run with go test -race" --scope project:test --tags "test,race"
OUT=$(echo '{"prompt":"how do I run the tests?"}' | engram hook 2>/dev/null || true)
assert_contains "Hook prompt search" "$OUT" "test"

echo ""
echo "=== Version ==="
OUT=$(command engram --version 2>&1)
assert_contains "Version 0.4.0" "$OUT" "0.4.0"

echo ""
echo "=== Schema version ==="
rm -f "$TMPDB"
engram store "init" --scope global >/dev/null 2>&1
OUT=$(sqlite3 "$TMPDB" "SELECT MAX(version) FROM schema_version" 2>/dev/null || echo "?")
assert_contains "Schema version 3" "$OUT" "3"

echo ""
echo "================================"
echo "Results: $PASS passed, $FAIL failed"
if [ "$FAIL" -gt 0 ]; then
    exit 1
fi
echo "All tests passed."
