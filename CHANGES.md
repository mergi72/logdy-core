# VFS Platform fork changes

This repository is a modified fork of `logdyhq/logdy-core`.

## 0.18.7 (2026-08-24)

- Initialize Web UI clients from the configured in-memory ring capacity instead of the previous hard-coded 100-message limit.
- Preserve live following while making the complete configured cache available to the debugger UI.

## 0.18.6 (2026-08-19)

- Embed Logdy UI 0.18.5 with taller hovered filter and facet sections.

## 0.18.5 (2026-08-19)

- Embed Logdy UI 0.18.4 with compact Origins labels and full-path tooltips.

## 0.18.4 (2026-08-19)

- Embed Logdy UI 0.18.3 with visible Next and Prev navigation buttons in the log detail drawer.
- Keep machine-specific VS Code task configuration out of the public repository.
- Decode non-UTF-8 Windows log lines as Windows-1250 so Czech text remains readable.

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
