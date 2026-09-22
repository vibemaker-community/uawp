# Identity and Sessions

UAWP separates a long-lived Worker identity from a single conversation or
execution Session. On first human `uawp resume`, the CLI suggests the static
format `Zhang San's Codex`; replace the person and Agent names yourself.

The display name is only a user-supplied label. Any valid non-control string is
allowed, duplicate labels are allowed, and labels never grant ownership.
Generated Worker and Session IDs are the actual identifiers.

```bash
uawp identity create --name "Sushi's Gemini"
uawp identity list
uawp identity use PROFILE_ID
uawp identity show
uawp identity remove PROFILE_ID
uawp session new --format json
```

A physical Workspace has one ACTIVE writing Session. Opening another
conversation does not silently inherit or replace it. Use normal handoff or
release before switching Sessions, then run `uawp resume` from the new Session.
Same-directory parallel writing is not supported. Git is optional.

If the old Session crashed and remains ACTIVE, UAWP never guesses that it is
stale. A Human Controller must inspect the exact Worker ID, Session ID, and
generation and use the reviewed stale-recovery operation.

Profiles and bindings are local configuration, outside `.uawp/`. Removing a
bound profile warns and removes only local records; it never edits a Workspace.
