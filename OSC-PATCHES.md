# OSC additions

This fork adds files only; no upstream source is modified, so syncing with
`Cisco-Talos/clamav` never conflicts.

- `Dockerfile.osc`: wraps the official `clamav/clamav:<version>_base` image.
- `osc-entrypoint.sh`: OSC defaults (stdout logging, `$PORT`), starts the scan API, then `exec`s upstream `/init`.
- `osc-scan-api/`: small Go HTTP front-end for clamd (OSC exposes HTTP only).
  `GET /` health, `GET /version`, `POST /scan` (multipart file(s) or raw body).
  `SCAN_MAX_BYTES` (default 100 MiB) should not exceed clamd's `StreamMaxLength`.

To bump ClamAV, change `CLAMAV_VERSION` in `Dockerfile.osc`.
