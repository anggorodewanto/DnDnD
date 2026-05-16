# Worker Report: H-H03

## Finding
ApproveASI passes feat-type choices to ApplyASI which rejects them with "unsupported ASI type: feat".

## Fix Applied

### 1. Added `Feat` field to `ASIChoice` (`internal/levelup/asi.go`)
Added `Feat FeatInfo` field so feat-type ASI choices can carry the full feat info needed by `ApplyFeat`.

### 2. Added feat-type branch in `ApproveASI` (`internal/levelup/service.go`)
Before calling `ApplyASI`, check if `choice.Type == ASIFeat`. If so, route to `s.ApplyFeat(ctx, characterID, choice.Feat)` instead.

### 3. Test added (`internal/levelup/service_test.go`)
`TestService_ApproveASI_FeatType_RoutesToApplyFeat` — verifies that `ApproveASI` with `Type=ASIFeat` succeeds and the feat is added to the character's features.

## Verification
- Red: test failed with `applying ASI: unsupported ASI type: feat`
- Green: test passes after adding the branch
- All tests pass (excluding `internal/database`)
