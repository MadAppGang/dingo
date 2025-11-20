#!/bin/bash

# Integration test runner for Phase 9
# Tests all golden files and creates comprehensive report

RESULTS_FILE="ai-docs/sessions/20251120-135205/02-implementation/T5-integration-results.md"
ISSUES_FILE="ai-docs/sessions/20251120-135205/02-implementation/T5-integration-issues.md"
STATUS_FILE="ai-docs/sessions/20251120-135205/02-implementation/T5-integration-status.txt"

# Initialize reports
echo "# Phase 9 Integration Test Results" > "$RESULTS_FILE"
echo "Generated: $(date)" >> "$RESULTS_FILE"
echo "" >> "$RESULTS_FILE"

echo "# Integration Test Issues" > "$ISSUES_FILE"
echo "Generated: $(date)" >> "$ISSUES_FILE"
echo "" >> "$ISSUES_FILE"

# Counters
TOTAL=0
TRANSPILE_SUCCESS=0
TRANSPILE_FAIL=0
COMPILE_SUCCESS=0
COMPILE_FAIL=0

echo "## Test Results by Feature" >> "$RESULTS_FILE"
echo "" >> "$RESULTS_FILE"

# Test each feature category
for category in error_prop null_coalesce lambda func_util; do
    echo "### ${category} Tests" >> "$RESULTS_FILE"
    echo "" >> "$RESULTS_FILE"

    for file in tests/golden/${category}_*.dingo; do
        if [ -f "$file" ]; then
            TOTAL=$((TOTAL + 1))
            basename=$(basename "$file" .dingo)
            echo -n "Testing $basename... "

            # Transpile
            if ./dingo build "$file" > /tmp/dingo_test.log 2>&1; then
                TRANSPILE_SUCCESS=$((TRANSPILE_SUCCESS + 1))
                echo "✓ Transpile" >> "$RESULTS_FILE"

                # Try to compile
                gofile="${file%.dingo}.go"
                if go build -o /dev/null "$gofile" 2>/dev/null; then
                    COMPILE_SUCCESS=$((COMPILE_SUCCESS + 1))
                    echo "  - $basename: ✓ Transpile ✓ Compile" >> "$RESULTS_FILE"
                    echo "PASS"
                else
                    COMPILE_FAIL=$((COMPILE_FAIL + 1))
                    echo "  - $basename: ✓ Transpile ✗ Compile" >> "$RESULTS_FILE"
                    echo "COMPILE FAIL"
                    echo "### $basename - Compile Error" >> "$ISSUES_FILE"
                    go build "$gofile" 2>&1 | head -10 >> "$ISSUES_FILE"
                    echo "" >> "$ISSUES_FILE"
                fi
            else
                TRANSPILE_FAIL=$((TRANSPILE_FAIL + 1))
                echo "  - $basename: ✗ Transpile" >> "$RESULTS_FILE"
                echo "TRANSPILE FAIL"
                echo "### $basename - Transpile Error" >> "$ISSUES_FILE"
                tail -5 /tmp/dingo_test.log >> "$ISSUES_FILE"
                echo "" >> "$ISSUES_FILE"
            fi
        fi
    done
    echo "" >> "$RESULTS_FILE"
done

# Test showcase
echo "### Showcase Comprehensive" >> "$RESULTS_FILE"
if [ -f "tests/golden/showcase_comprehensive.dingo" ]; then
    TOTAL=$((TOTAL + 1))
    if ./dingo build tests/golden/showcase_comprehensive.dingo > /tmp/dingo_test.log 2>&1; then
        TRANSPILE_SUCCESS=$((TRANSPILE_SUCCESS + 1))
        if go build -o /dev/null tests/golden/showcase_comprehensive.go 2>/dev/null; then
            COMPILE_SUCCESS=$((COMPILE_SUCCESS + 1))
            echo "  - showcase_comprehensive: ✓ Transpile ✓ Compile" >> "$RESULTS_FILE"
        else
            COMPILE_FAIL=$((COMPILE_FAIL + 1))
            echo "  - showcase_comprehensive: ✓ Transpile ✗ Compile" >> "$RESULTS_FILE"
            echo "### showcase_comprehensive - Compile Error" >> "$ISSUES_FILE"
            go build tests/golden/showcase_comprehensive.go 2>&1 | head -10 >> "$ISSUES_FILE"
        fi
    else
        TRANSPILE_FAIL=$((TRANSPILE_FAIL + 1))
        echo "  - showcase_comprehensive: ✗ Transpile" >> "$RESULTS_FILE"
        echo "### showcase_comprehensive - Transpile Error" >> "$ISSUES_FILE"
        tail -5 /tmp/dingo_test.log >> "$ISSUES_FILE"
    fi
fi
echo "" >> "$RESULTS_FILE"

# Summary
echo "## Summary Statistics" >> "$RESULTS_FILE"
echo "" >> "$RESULTS_FILE"
echo "- Total Tests: $TOTAL" >> "$RESULTS_FILE"
echo "- Transpile Success: $TRANSPILE_SUCCESS" >> "$RESULTS_FILE"
echo "- Transpile Failures: $TRANSPILE_FAIL" >> "$RESULTS_FILE"
echo "- Compile Success: $COMPILE_SUCCESS" >> "$RESULTS_FILE"
echo "- Compile Failures: $COMPILE_FAIL" >> "$RESULTS_FILE"
echo "" >> "$RESULTS_FILE"

TRANSPILE_RATE=$(echo "scale=1; $TRANSPILE_SUCCESS * 100 / $TOTAL" | bc)
COMPILE_RATE=$(echo "scale=1; $COMPILE_SUCCESS * 100 / $TOTAL" | bc)

echo "- Transpile Success Rate: ${TRANSPILE_RATE}%" >> "$RESULTS_FILE"
echo "- End-to-End Success Rate: ${COMPILE_RATE}%" >> "$RESULTS_FILE"

# Status file
if [ $TRANSPILE_FAIL -eq 0 ] && [ $COMPILE_FAIL -eq 0 ]; then
    echo "SUCCESS: All $TOTAL tests passing (transpile + compile)" > "$STATUS_FILE"
else
    echo "FAILURES: $TRANSPILE_FAIL transpile failures, $COMPILE_FAIL compile failures out of $TOTAL tests" > "$STATUS_FILE"
fi

echo "Done! Results in $RESULTS_FILE"
cat "$STATUS_FILE"
