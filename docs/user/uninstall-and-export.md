# Uninstall and verified export

Default uninstall detaches every configured provider but preserves `.uawp/`:

```bash
uawp uninstall --workspace /absolute/project --format json
uawp uninstall --workspace /absolute/project --approve PLAN_ID --format json
```

Only exact UAWP imports and managed blocks are removed. A native file is deleted only when UAWP created the whole file, its full hash still matches, and removal leaves it empty. User additions are preserved.

Purge is a separate destructive operation and requires a new external archive path:

```bash
uawp uninstall --purge-state --export /absolute/backups/project-uawp.tar.gz \
  --workspace /absolute/project --format json
uawp uninstall --purge-state --export /absolute/backups/project-uawp.tar.gz \
  --workspace /absolute/project --approve PLAN_ID --format json
```

UAWP detaches providers, snapshots `.uawp/`, writes a deterministic tar/gzip archive without overwriting an existing file, reopens it, verifies entry hashes, and only then commits namespace removal through a reserved tombstone. Keep the archive for manual restore: extract its `.uawp/` tree into an empty target project after reviewing `export-manifest.json`.
