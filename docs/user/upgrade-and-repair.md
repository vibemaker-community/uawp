# Upgrade and repair

UAWP state upgrades and repairs use the same immutable preview/approval flow as initialization. Preview is read-only and exits `5`:

```bash
uawp upgrade --workspace /absolute/project --format json
uawp upgrade --workspace /absolute/project --approve PLAN_ID --format json
uawp repair --workspace /absolute/project --format json
uawp repair --workspace /absolute/project --approve PLAN_ID --format json
```

The supported target is exactly `1.2.0`. Valid RELEASED `1.0.0` state migrates
through the composed path, and valid RELEASED `1.1.0` state migrates directly.
ACTIVE sources, unknown minor versions, and downgrades stop. A successful
migration writes receipts under `.uawp/migrations/` and writes the final
`manifest.json` once, last.

Automatic repair is intentionally narrow. It may recreate the exact current-version generated `INSTRUCTIONS.md` or repair an integration only when ownership markers, outside bytes, manifest registration, and adapter evidence agree. Missing `CONTEXT.md` or `DECISIONS.md`, malformed markers, outside-content drift, binary or linked entries, and ambiguous registrations require human restoration.

Upgrade and repair require `RELEASED` ownership and no unresolved transaction.
