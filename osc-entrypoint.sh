#!/bin/bash
# OSC entrypoint for ClamAV.
# clamd only speaks raw TCP (port 3310) and OSC only exposes HTTP, so osc-scan-api
# is started on $PORT as an HTTP front-end (health, /scan, /version). All upstream tuning stays available through
# CLAMD_CONF_<Option> and FRESHCLAM_CONF_<Option> env vars (handled by /init).
set -eu

PORT="${PORT:-8080}"

# Defaults (can be overridden by setting the variable)
export CLAMD_CONF_TCPSocket="${CLAMD_CONF_TCPSocket:-3310}"
export CLAMD_CONF_TCPAddr="${CLAMD_CONF_TCPAddr:-0.0.0.0}"
export CLAMD_CONF_LogFile="${CLAMD_CONF_LogFile:-/dev/stdout}"
export CLAMD_CONF_LogTime="${CLAMD_CONF_LogTime:-yes}"
export FRESHCLAM_CONF_UpdateLogFile="${FRESHCLAM_CONF_UpdateLogFile:-/dev/stdout}"

# HTTP scan API (health on /, POST /scan, GET /version); talks to local clamd
PORT="$PORT" CLAMD_ADDR="127.0.0.1:${CLAMD_CONF_TCPSocket}" /usr/local/bin/osc-scan-api &

exec "$@"
