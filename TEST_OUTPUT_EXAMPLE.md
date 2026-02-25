# Test Output Examples

## Running Unit Tests

```bash
$ go test ./internal/decoder/suggestions_test.go -v

=== RUN   TestNewSuggestionEngine
--- PASS: TestNewSuggestionEngine (0.00s)

=== RUN   TestAnalyzeEvents_UninitializedContract
--- PASS: TestAnalyzeEvents_UninitializedContract (0.00s)

=== RUN   TestAnalyzeEvents_MissingAuthorization
--- PASS: TestAnalyzeEvents_MissingAuthorization (0.00s)

=== RUN   TestAnalyzeEvents_InsufficientBalance
--- PASS: TestAnalyzeEvents_InsufficientBalance (0.00s)

=== RUN   TestAnalyzeEvents_InvalidParameters
--- PASS: TestAnalyzeEvents_InvalidParameters (0.00s)

=== RUN   TestAnalyzeEvents_ContractNotFound
--- PASS: TestAnalyzeEvents_ContractNotFound (0.00s)

=== RUN   TestAnalyzeEvents_ResourceLimitExceeded
--- PASS: TestAnalyzeEvents_ResourceLimitExceeded (0.00s)

=== RUN   TestAnalyzeEvents_NoMatch
--- PASS: TestAnalyzeEvents_NoMatch (0.00s)

=== RUN   TestAnalyzeCallTree
--- PASS: TestAnalyzeCallTree (0.00s)

=== RUN   TestFormatSuggestions
--- PASS: TestFormatSuggestions (0.00s)

=== RUN   TestFormatSuggestions_Empty
--- PASS: TestFormatSuggestions_Empty (0.00s)

=== RUN   TestAddCustomRule
--- PASS: TestAddCustomRule (0.00s)

=== RUN   TestAnalyzeEvents_DuplicateRules
--- PASS: TestAnalyzeEvents_DuplicateRules (0.00s)

PASS
ok      github.com/dotandev/hintents/internal/decoder    0.123s
```

## Running Integration Tests

```bash
$ go test ./internal/decoder/integration_test.go -v

=== RUN   TestIntegration_SuggestionEngineWithDecoder
=== RUN   TestIntegration_SuggestionEngineWithDecoder/UninitializedContract
--- PASS: TestIntegration_SuggestionEngineWithDecoder/UninitializedContract (0.00s)
=== RUN   TestIntegration_SuggestionEngineWithDecoder/NestedCallsWithMultipleErrors
--- PASS: TestIntegration_SuggestionEngineWithDecoder/NestedCallsWithMultipleErrors (0.00s)
=== RUN   TestIntegration_SuggestionEngineWithDecoder/SuccessfulTransaction
--- PASS: TestIntegration_SuggestionEngineWithDecoder/SuccessfulTransaction (0.00s)
--- PASS: TestIntegration_SuggestionEngineWithDecoder (0.00s)

=== RUN   TestIntegration_CustomRuleWorkflow
--- PASS: TestIntegration_CustomRuleWorkflow (0.00s)

=== RUN   TestIntegration_RealWorldScenario
    integration_test.go:XXX: Suggestions for junior developer:
        
        === Potential Fixes (Heuristic Analysis) ===
        ⚠️  These are suggestions based on common error patterns. Always verify before applying.
        
        1. 🔴 [Confidence: high]
           Potential Fix: Ensure you have called initialize() on this contract before invoking other functions.
--- PASS: TestIntegration_RealWorldScenario (0.00s)

PASS
ok      github.com/dotandev/hintents/internal/decoder    0.089s
```

## CLI Output Example 1: Uninitialized Contract

```bash
$ erst debug 5c0a1234567890abcdef1234567890abcdef1234567890abcdef1234567890ab --network testnet

Debugging transaction: 5c0a1234567890abcdef1234567890abcdef1234567890abcdef1234567890ab
Primary Network: testnet
Fetching transaction: 5c0a1234567890abcdef1234567890abcdef1234567890abcdef1234567890ab
Transaction fetched successfully. Envelope size: 1024 bytes
Running simulation on testnet...

--- Result for testnet ---
Status: failed
Error: Contract execution failed

Resource Usage:
  CPU Instructions: 45678 / 100000000 (0.05%)
  Memory Bytes: 2048 / 41943040 (0.00%)
  Operations: 12

Events: 8, Logs: 3

=== Potential Fixes (Heuristic Analysis) ===
⚠️  These are suggestions based on common error patterns. Always verify before applying.

1. 🔴 [Confidence: high]
   Potential Fix: Ensure you have called initialize() on this contract before invoking other functions.

=== Security Analysis ===
[OK] No security issues detected

Token Flow Summary:
  → XLM transferred

Session created: abc123-def456
Run 'erst session save' to persist this session.
```

## CLI Output Example 2: Multiple Errors

```bash
$ erst debug abc123def456... --network testnet

Debugging transaction: abc123def456...
Primary Network: testnet
Fetching transaction: abc123def456...
Transaction fetched successfully. Envelope size: 2048 bytes
Running simulation on testnet...

--- Result for testnet ---
Status: failed
Error: Multiple operation failures

Resource Usage:
  CPU Instructions: 89234567 / 100000000 (89.23%) [!]  WARNING
  Memory Bytes: 38000000 / 41943040 (90.60%) [!]  WARNING
  Operations: 45

Events: 23, Logs: 12

=== Potential Fixes (Heuristic Analysis) ===
⚠️  These are suggestions based on common error patterns. Always verify before applying.

1. 🔴 [Confidence: high]
   Potential Fix: Verify that all required signatures are present and the invoker has proper authorization.

2. 🔴 [Confidence: high]
   Potential Fix: Ensure the account has sufficient balance to cover the transaction and maintain minimum reserves.

3. 🟡 [Confidence: medium]
   Potential Fix: Optimize your contract code to reduce CPU/memory usage, or increase resource limits in the transaction.

=== Security Analysis ===
[!]  VERIFIED SECURITY RISKS: 1
* HEURISTIC WARNINGS: 2

Findings:
1. [!] [verified_risk] high - Unauthorized Access Attempt
   Attempted to access restricted function without proper authorization
   Evidence: auth_check failed in contract abc123

2. * [heuristic_warning] medium - High Resource Usage
   Transaction is using 89% of CPU limit
   
3. * [heuristic_warning] low - Multiple Failed Operations
   3 out of 5 operations failed

Token Flow Summary:
  → 100 XLM attempted transfer (failed)
  → 50 USDC attempted transfer (failed)

Session created: abc123-def456
Run 'erst session save' to persist this session.
```

## CLI Output Example 3: Success (No Suggestions)

```bash
$ erst debug xyz789... --network testnet

Debugging transaction: xyz789...
Primary Network: testnet
Fetching transaction: xyz789...
Transaction fetched successfully. Envelope size: 512 bytes
Running simulation on testnet...

--- Result for testnet ---
Status: success

Resource Usage:
  CPU Instructions: 12345 / 100000000 (0.01%)
  Memory Bytes: 1024 / 41943040 (0.00%)
  Operations: 3

Events: 5, Logs: 2

=== Security Analysis ===
[OK] No security issues detected

Token Flow Summary:
  → 10 XLM transferred successfully
  Alice → Bob: 10 XLM

Session created: xyz789-abc123
Run 'erst session save' to persist this session.
```

## CLI Output Example 4: Custom Rule

```bash
$ erst debug custom123... --network testnet

Debugging transaction: custom123...
Primary Network: testnet
Fetching transaction: custom123...
Transaction fetched successfully. Envelope size: 768 bytes
Running simulation on testnet...

--- Result for testnet ---
Status: failed
Error: Rate limit exceeded

Resource Usage:
  CPU Instructions: 5678 / 100000000 (0.01%)
  Memory Bytes: 512 / 41943040 (0.00%)
  Operations: 2

Events: 4, Logs: 1

=== Potential Fixes (Heuristic Analysis) ===
⚠️  These are suggestions based on common error patterns. Always verify before applying.

1. 🔴 [Confidence: high]
   Potential Fix: Wait before retrying or implement exponential backoff in your application.

=== Security Analysis ===
[OK] No security issues detected

Session created: custom123-xyz789
Run 'erst session save' to persist this session.
```

## Test Coverage Report

```bash
$ go test -cover ./internal/decoder/

ok      github.com/dotandev/hintents/internal/decoder    0.234s  coverage: 92.5% of statements
```

## Benchmark Results (Optional)

```bash
$ go test -bench=. ./internal/decoder/

goos: linux
goarch: amd64
pkg: github.com/dotandev/hintents/internal/decoder
BenchmarkAnalyzeEvents-8              50000     23456 ns/op     4096 B/op     42 allocs/op
BenchmarkAnalyzeCallTree-8            30000     45678 ns/op     8192 B/op     89 allocs/op
BenchmarkFormatSuggestions-8         100000     12345 ns/op     2048 B/op     21 allocs/op
PASS
ok      github.com/dotandev/hintents/internal/decoder    4.567s
```

## License

Copyright 2025 Erst Users  
SPDX-License-Identifier: Apache-2.0
