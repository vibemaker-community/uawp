# Security Policy

## Supported versions

Before the first stable release, only the latest commit on `main` is supported.
After v1.0.0, the latest stable minor line receives security fixes unless a
release notice states otherwise.

## Report a vulnerability privately

Use GitHub's private vulnerability reporting form:

<https://github.com/vibemaker-community/uawp/security/advisories/new>

Do not open a public issue. Include the affected version or commit, operating
system, reproduction steps, impact, and any suggested mitigation. Remove
credentials, private conversation content, and unrelated workspace data.

We will acknowledge a usable report when it is reviewed, coordinate validation
and remediation privately, and credit reporters who request credit. No fixed
response-time guarantee is made before a staffed security team exists.

## Scope

High-priority areas include destructive workspace writes, managed-block
escape, ownership or arbitration bypass, approval drift, archive traversal,
installer integrity, release provenance, and workflow credential exposure.
