# Feature: Heuristic-Based Error Suggestion Engine

## Summary

Implemented a heuristic-based error suggestion engine that analyzes Soroban transaction failures and provides actionable suggestions to help developers, especially junior developers, understand and fix common errors.

## Implementation Details

### Files Created

1. **internal/decoder/suggestions.go** (200+ lines)
   - Core suggestion engine implementation
   - 7 built-in heuristic rules
   - Support for custom rules
   - Event and call tree analysis

2. **internal/decoder/suggestions_test.go** (300+ lines)
   - Comprehensive test coverage
   - Tests for all built-in rules
   - Edge case handling
   - Custom rule testing

3. **internal/decoder/integration_test.go** (200+ lines)
   - End-to-end integration tests
   - Real-world scenario simulations
   - Custom rule workflow tests

4. **internal/decoder/suggestions_example.go**
   - Usage examples
   - Code samples for developers

5. **docs/ERROR_SUGGESTIONS.md**
   - Complete documentation
   - Rule descriptions
   - Usage guide
   - Best practices

### Files Modified

1. **internal/cmd/debug.go**
   - Integrated suggestion engine into debug command
   - Displays suggestions before security analysis
   - Added decoder package import

2. **README.md**
   - Added error suggestions to core features
   - Added documentation link

## Built-in Rules

1. **Uninitialized Contract** (High Confidence)
   - Detects: Empty storage, missing data entries
   - Suggests: Call initialize() before other functions

2. **Missing Authorization** (High Confidence)
   - Detects: Auth failures, unauthorized access
   - Suggests: Verify signatures and permissions

3. **Insufficient Balance** (High Confidence)
   - Detects: Balance errors, underfunded accounts
   - Suggests: Ensure sufficient balance and reserves

4. **Invalid Parameters** (Medium Confidence)
   - Detects: Malformed or invalid parameters
   - Suggests: Check parameter types and constraints

5. **Contract Not Found** (High Confidence)
   - Detects: Missing or invalid contract IDs
   - Suggests: Verify contract deployment

6. **Resource Limit Exceeded** (Medium Confidence)
   - Detects: CPU/memory limit violations
   - Suggests: Optimize code or increase limits

7. **Reentrancy Detected** (Medium Confidence)
   - Detects: Recursive call patterns
   - Suggests: Implement reentrancy guards

## Usage Example

```bash
$ erst debug <tx-hash> --network testnet

Debugging transaction: abc123...
Transaction fetched successfully. Envelope size: 256 bytes

--- Result for testnet ---
Status: failed
Error: Contract execution failed

=== Potential Fixes (Heuristic Analysis) ===
⚠️  These are suggestions based on common error patterns. Always verify before applying.

1. 🔴 [Confidence: high]
   Potential Fix: Ensure you have called initialize() on this contract before invoking other functions.

2. 🟡 [Confidence: medium]
   Potential Fix: Check that all function parameters match the expected types and constraints.

=== Security Analysis ===
...
```

## Testing

All tests pass with comprehensive coverage:

```bash
go test ./internal/decoder/suggestions_test.go -v
go test ./internal/decoder/integration_test.go -v
```

Test coverage includes:
- [OK] All 7 built-in rules
- [OK] Custom rule addition
- [OK] Call tree analysis
- [OK] Deduplication logic
- [OK] Output formatting
- [OK] Edge cases (empty events, no matches)
- [OK] Real-world scenarios

## Success Criteria

[OK] **CLI prints suggestions**: Integrated into `erst debug` command  
[OK] **Clearly marked**: All suggestions labeled as "Potential Fixes"  
[OK] **Heuristic rules**: 7 built-in rules for common errors  
[OK] **Extensible**: Support for custom rules  
[OK] **Well-tested**: Comprehensive test suite  
[OK] **Documented**: Complete documentation with examples  
[OK] **Junior-friendly**: Clear, actionable suggestions

## Example Commit Message

```
feat(decoder): implement heuristic-based error suggestion engine

Add a suggestion engine that analyzes Soroban transaction failures
and provides actionable fixes for common errors. This helps junior
developers understand why transactions fail and how to fix them.

Features:
- 7 built-in heuristic rules for common Soroban errors
- Confidence levels (high, medium, low) for each suggestion
- Support for custom rules
- Integration with erst debug command
- Comprehensive test coverage

Rules include:
- Uninitialized contract detection
- Missing authorization
- Insufficient balance
- Invalid parameters
- Contract not found
- Resource limit exceeded
- Reentrancy detection

Example output:
  === Potential Fixes (Heuristic Analysis) ===
  1. 🔴 [Confidence: high]
     Potential Fix: Ensure you have called initialize() on this contract

Closes #<issue-number>
```

## Branch

Suggested branch name: `feature/decoder-suggestions`

## Next Steps

1. Create feature branch
2. Commit changes with message above
3. Run full test suite: `make test`
4. Run linter: `make lint`
5. Create PR with:
   - Link to this document
   - Screenshots of CLI output
   - Test results
   - Documentation updates

## Future Enhancements

- Machine learning-based pattern detection
- Integration with contract source code
- Suggestion ranking based on accuracy
- Community-contributed rule database
- Multi-language support
- Interactive suggestion refinement

## Notes for Reviewers

- All suggestions are clearly marked as heuristic-based
- Confidence levels help users prioritize
- Engine is extensible for project-specific rules
- No breaking changes to existing code
- Comprehensive test coverage
- Well-documented with examples

## License

Copyright 2025 Erst Users  
SPDX-License-Identifier: Apache-2.0
