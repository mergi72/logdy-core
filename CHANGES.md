# VFS Platform fork changes

This repository is a modified fork of `logdyhq/logdy-core`.

## 0.18.3 (2026-08-19)

- Hide trusted configuration and operational status details from unauthenticated clients.
- Remove expired sessions during authentication checks.
- Expand race-detector coverage to the complete HTTP package.
- Make the shared bulk-window setting race-safe and synchronize buffer assertions in tests.
- Close configuration files immediately after reading so Windows does not retain a file lock.

## 0.18.2 (2026-08-19)

- Added Apache-2.0 modification notices and documented fork provenance.
- Retained the original Apache-2.0 license.

## Earlier VFS modifications

- Replaced password-bearing URLs with same-origin session authentication.
- Added WebSocket origin validation, protected configuration APIs, secure client IDs, and concurrency hardening.
- Added safer HTTP timeouts and prefix-aware embedded UI paths.
- Added VFS log parsing, RUN/STOP controls, and the modified embedded Logdy UI.
- Expanded security, concurrency, cross-platform, and CI coverage.
