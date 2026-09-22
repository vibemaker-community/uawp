# Stale ACTIVE recovery

UAWP never decides that an ACTIVE claim is stale. Time passing or process absence cannot release ownership.
This applies equally to another Agent and another Session of the same Agent.
The blocked Session remains read-only until a Human Controller reviews and
explicitly approves recovery.

```sh
./uawp recover --workspace ./project --controller-id human-1 --reason "confirmed worker crash" --format json
```

After reviewing the preview, rerun the identical command with `--approve <planID>`. The token approves that exact plan; it does not authenticate the person's identity. Ownership or decisions drift invalidates it, and successful recovery makes replay fail.

Recovery records the arbitration and releases the old claim. It does not acquire ownership for a replacement worker; that worker must use normal `acquire` separately.
