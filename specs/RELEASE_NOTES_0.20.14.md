# skillshare v0.20.14 Release Notes

## TL;DR

1. **Push failure errors now redact token-auth URLs** — failed push output is sanitized before it reaches CLI, API, or dashboard callers.
2. **Push diagnostics stay useful** — Skillshare still preserves Git and pre-push hook messages, including cases where Git only prints a generic "failed to push some refs" summary.

## Bug fix: push failures redact tokens without hiding diagnostics

When Skillshare pushes to a Git remote using token-based authentication, a failed push can include the rewritten credential-bearing URL in Git's error output. That output is now sanitized before being shown to users or returned through API/UI callers, so token values are replaced instead of leaked.

The fix also keeps the useful part of the failure. If Git or a pre-push hook explains why the push was rejected, Skillshare preserves that diagnostic text instead of reducing the error to a generic push failure.

Refs: #214.
