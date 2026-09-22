# Transaction recovery

Transaction recovery handles an interrupted UAWP mutation. It is different from `uawp recover`, which only arbitrates stale `ACTIVE_WORKER.md` ownership.

```bash
uawp transaction status --workspace /absolute/project --format json
uawp transaction rollback --workspace /absolute/project \
  --controller-id human-1 --reason "verified rollback" --format json
uawp transaction continue --workspace /absolute/project \
  --controller-id human-1 --reason "verified continuation" --format json
```

Rollback and continue first return exit `5` plus a plan ID. Repeat the same command with `--approve PLAN_ID`. Controller ID and reason are audit assertions, not permission bypasses. UAWP only offers actions supported by the persisted plan, backups, journal states, and current filesystem hashes.

Classifications are `ROLLBACK_AVAILABLE`, `CONTINUE_AVAILABLE`, `ROLLBACK_OR_CONTINUE_AVAILABLE`, `TRANSACTION_COMPLETE`, and `MANUAL_RECOVERY_REQUIRED`. Manual recovery means evidence is incomplete, unsafe, or contradicted by later user changes; UAWP does not guess.

Exit codes: `0` success/read-only result, `2` usage, `3` unsafe namespace, `4` invalid or recovery-required state, `5` explicit approval required, and `10` internal/environment failure.
