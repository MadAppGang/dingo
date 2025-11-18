# Testing Skill

You are executing the **Testing** pattern. This skill helps you delegate testing tasks to specialized test agents while keeping main chat focused on results.

## Your Task

The user wants to run tests, create tests, or fix failing tests. Follow these steps:

### Step 1: Identify Testing Scope

Determine what kind of testing:
- **Run existing tests**: Execute test suite and report results
- **Create new tests**: Write golden tests for new feature
- **Fix failing tests**: Debug and fix test failures
- **Test specific feature**: Run subset of tests

### Step 2: Create Output Location

```bash
# For simple test runs (just results)
mkdir -p ai-docs/test-results/

# For complex test work (creation, debugging)
SESSION=$(date +%Y%m%d-%H%M%S)
mkdir -p ai-docs/sessions/$SESSION/output/
```

### Step 3: Choose Appropriate Agent

Based on domain:
- **Go tests** (golden tests, unit tests) → `golang-tester`
- **Astro/React tests** → `astro-developer` (no specialized tester yet)
- **General code** → `general-purpose` (last resort)

### Step 4: Delegate to Test Agent

#### Running Existing Tests

```
Task tool → golang-tester:

Run tests: [scope]

Scope:
- All tests: go test ./...
- Specific: go test ./tests -run TestGoldenFiles
- Package: go test ./pkg/...

Your Tasks:
1. Run the test suite
2. Capture output (pass/fail counts, failures)
3. If failures: Identify what failed and why
4. Write results to: ai-docs/sessions/$SESSION/output/test-results.md

Return to Main Chat (MAX 5 sentences):
Status: All Pass / Some Fail / Error
Pass rate: X/Y (Z%)
Failures: [count] (or "none")
Key issue: [brief if failures]
Details: ai-docs/sessions/$SESSION/output/test-results.md

DO NOT return full test output in response.
```

#### Creating New Tests

```
Task tool → golang-tester:

Create golden tests for: [feature]

Requirements:
- Feature: [feature name]
- Test files: tests/golden/[feature]_NN_[description].dingo
- Follow: tests/golden/GOLDEN_TEST_GUIDELINES.md
- Include: Both .dingo and .go.golden files

Your Tasks:
1. Read golden test guidelines
2. Create appropriate test cases (01=basic, 02-03=intermediate, 04+=advanced)
3. Write .dingo source files
4. Generate .go.golden files (run transpiler)
5. Verify tests pass
6. Write summary to: ai-docs/sessions/$SESSION/output/test-creation-summary.md

Return to Main Chat (MAX 5 sentences):
Status: Success/Partial/Failed
Tests created: [count] test files
Coverage: [what scenarios covered]
Pass rate: [if run]
Details: ai-docs/sessions/$SESSION/output/test-creation-summary.md

DO NOT return test code in response.
```

#### Fixing Failing Tests

```
Task tool → golang-tester:

Fix failing tests: [which tests]

Context:
- Failing tests: [list or "see test results"]
- Recent changes: [what changed that might have broken tests]

Your Tasks:
1. Run tests to reproduce failures
2. Analyze why tests are failing
3. Fix the issues (update tests or fix code)
4. Verify all tests pass
5. Write summary to: ai-docs/sessions/$SESSION/output/test-fix-summary.md

Return to Main Chat (MAX 5 sentences):
Status: Fixed/Partial/Blocked
Tests fixed: [count]
Root cause: [brief]
Pass rate: X/Y (Z%)
Details: ai-docs/sessions/$SESSION/output/test-fix-summary.md

DO NOT return fix details in response.
```

### Step 5: Present Results to User

After receiving agent summary:

**For test runs**:
1. Show pass/fail summary
2. Highlight any failures
3. Provide path to detailed results
4. Offer to fix failures if any

**For test creation**:
1. Show count of tests created
2. Mention scenarios covered
3. Show pass rate
4. Provide file paths

**For test fixes**:
1. Show fix status
2. Mention root cause
3. Show updated pass rate
4. Provide details path

### Example Execution

```
User: "Run all golden tests"

You (main chat):
1. Create session: ai-docs/sessions/20251118-162000/
2. Delegate to golang-tester: "Run all golden tests"
3. Receive summary:
   "Status: Some Fail
    Pass rate: 261/267 (97.8%)
    Failures: 6 tests (option inference issues)
    Key issue: None constant not inferred in all contexts
    Details: ai-docs/sessions/20251118-162000/output/test-results.md"
4. Present to user:
   "Test run complete: 261/267 passing (97.8%)
    6 failing tests related to Option None inference
    Want me to investigate and fix these failures?"

Total context: ~15 lines
Detailed output: In session file
```

### Step 6: Optional Follow-Up

Based on results:
- **All tests pass**: Celebrate! Ask if user wants coverage report
- **Some tests fail**: Offer to fix or investigate
- **Tests error**: Delegate investigation to golang-developer

## Key Rules

1. ✅ **Always delegate testing** to golang-tester (for Go)
2. ✅ **Request file output** (not inline test results)
3. ✅ **Follow golden test guidelines** when creating tests
4. ✅ **Report pass/fail summary** clearly
5. ✅ **Offer to fix failures** proactively
6. ❌ **Never show full test output** in main chat (>50 lines)
7. ❌ **Never create tests directly** (delegate to agent)
8. ❌ **Never ignore test failures** (offer to fix)

## Golden Test Guidelines

When creating tests, agent MUST follow:
- **Location**: `tests/golden/`
- **Naming**: `{feature}_{NN}_{description}.dingo`
- **Pairs**: Both `.dingo` and `.go.golden` required
- **Guidelines**: `tests/golden/GOLDEN_TEST_GUIDELINES.md`

**Feature prefixes**:
- `error_prop_` - Error propagation
- `result_` - Result<T,E> type
- `option_` - Option<T> type
- `sum_types_` - Enums/sum types
- `pattern_match_` - Pattern matching
- `lambda_` - Lambda functions

## Success Metrics

- **Context saved**: 10x reduction (summary vs full output)
- **Clarity**: Pass/fail immediately visible
- **Actionable**: Offer to fix failures automatically

## What to Return to User

1. Pass/fail summary (1 line)
2. Pass rate percentage
3. Failure count and brief reason
4. Session folder path
5. Offer to fix/investigate

**Keep it simple. Tests pass or they don't. Let agents handle details!**
