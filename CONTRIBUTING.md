# Contributing to UAWP

Thank you for helping improve UAWP. The canonical repository is
<https://github.com/vibemaker-community/uawp>.

## Before opening a pull request

1. Open or reference an issue for behavior changes.
2. Keep the Agent-neutral Core independent of any provider.
3. Preserve `.uawp/` namespace ownership and non-destructive integration.
4. Add a failing test before changing behavior, then make the full suite pass.
5. Update user documentation when commands or guarantees change.
6. Cite the provider's official documentation for adapter behavior.
7. Complete the [Individual CLA](CLA.md) through CLA Assistant.

Run the local checks with:

```sh
make check
```

Never include credentials, private workspace state, private conversation
content, personal data, or proprietary project files in an issue, test
fixture, log, or pull request.

## Contribution ownership

You retain copyright in your Contribution. By submitting it, you agree to the
Individual CLA in addition to GPL-3.0-only. Contributions owned by or made on
behalf of a company are paused until an Entity CLA is available.

Li Rui may offer separately negotiated proprietary licenses for UAWP. The CLA
provides the rights needed to maintain that dual-licensing option.

## Review requirements

A change is ready to merge only after required CI and CLA checks pass and the
maintainer accepts its tests, documentation, safety impact, and compatibility.
Release-sensitive files require owner review.
