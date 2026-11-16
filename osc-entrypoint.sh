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

# Keep the HTTP upload limit in step with clamd's StreamMaxLength (e.g. 100M, 2G,
# or plain bytes) unless SCAN_MAX_BYTES is set explicitly.
if [ -z "${SCAN_MAX_BYTES:-}" ] && [ -n "${CLAMD_CONF_StreamMaxLength:-}" ]; then
  v="${CLAMD_CONF_StreamMaxLength}"
  num="${v%[KkMmGg]}"
  suf="${v#"$num"}"
  mult=""
  case "$suf" in
    "") mult=1 ;;
    K|k) mult=1024 ;;
    M|m) mult=1048576 ;;
    G|g) mult=1073741824 ;;
  esac
  if [[ "$num" =~ ^[0-9]+$ && -n "$mult" ]]; then
    export SCAN_MAX_BYTES=$((num * mult))
  else
    echo "osc-entrypoint: cannot parse StreamMaxLength '$v', using default HTTP upload limit" >&2
  fi
fi

# HTTP scan API (health on /, POST /scan, GET /version); talks to local clamd
PORT="$PORT" CLAMD_ADDR="127.0.0.1:${CLAMD_CONF_TCPSocket}" /usr/local/bin/osc-scan-api &

exec "$@"
