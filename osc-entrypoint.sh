#!/bin/bash
# OSC entrypoint for ClamAV.
# clamd only speaks raw TCP (port 3310), so a tiny HTTP stub is started on $PORT
# for the platform's health check. All upstream tuning stays available through
# CLAMD_CONF_<Option> and FRESHCLAM_CONF_<Option> env vars (handled by /init).
set -eu

PORT="${PORT:-8080}"

# Defaults (can be overridden by setting the variable)
export CLAMD_CONF_TCPSocket="${CLAMD_CONF_TCPSocket:-3310}"
export CLAMD_CONF_TCPAddr="${CLAMD_CONF_TCPAddr:-0.0.0.0}"
export CLAMD_CONF_LogFile="${CLAMD_CONF_LogFile:-/dev/stdout}"
export CLAMD_CONF_LogTime="${CLAMD_CONF_LogTime:-yes}"
export FRESHCLAM_CONF_UpdateLogFile="${FRESHCLAM_CONF_UpdateLogFile:-/dev/stdout}"

# HTTP health stub
WWW=/tmp/www
mkdir -p "$WWW"
cat > "$WWW/index.html" <<HTML
<html><body><h1>ClamAV</h1>
<p>clamd is listening on TCP port 3310. Use clamdscan, clamd-compatible clients or <code>INSTREAM</code> to scan files.</p>
</body></html>
HTML
httpd -f -p "0.0.0.0:${PORT}" -h "$WWW" &

exec "$@"
