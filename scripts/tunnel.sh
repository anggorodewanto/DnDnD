#!/usr/bin/env bash
# Expose the local DnDnD app (localhost:8080) over a public URL via a cloudflared
# "quick tunnel", so a remote player can reach the web character builder and the
# Discord OAuth login from their own machine.
#
#   scripts/tunnel.sh up      Download cloudflared if missing, start the tunnel,
#                             point .env (BASE_URL + OAUTH_REDIRECT_URL) at it,
#                             restart the app, and print the OAuth callback to
#                             register in the Discord dev portal.
#   scripts/tunnel.sh down    Stop the tunnel, restore .env, restart the app.
#   scripts/tunnel.sh status  Show whether a tunnel is running and its URL.
#   scripts/tunnel.sh url     Print the current tunnel URL (empty if down).
#
# The trycloudflare.com URL is EPHEMERAL: every `up` mints a fresh one, so the
# OAuth redirect must be re-registered in the Discord dev portal each time
# (Discord rejects unlisted redirect URIs -> login fails without it).
#
# No cloudflared account/authtoken is required (quick tunnels are anonymous).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

PORT="${APP_PORT:-8080}"
STATE_DIR="$ROOT/.tunnel"
PID_FILE="$STATE_DIR/cloudflared.pid"
LOG_FILE="$STATE_DIR/cloudflared.log"
URL_FILE="$STATE_DIR/url"
ENV_FILE="$ROOT/.env"
ENV_BACKUP="$ROOT/.env.bak.preTunnel"
CALLBACK_PATH="/portal/auth/callback"
CF_BIN=""

log() { printf '\033[36m[tunnel]\033[0m %s\n' "$*"; }
err() { printf '\033[31m[tunnel] ERROR:\033[0m %s\n' "$*" >&2; }

# resolve_cloudflared sets CF_BIN, preferring a system install, then bin/, then
# downloading the official release binary into bin/.
resolve_cloudflared() {
  if command -v cloudflared >/dev/null 2>&1; then CF_BIN="$(command -v cloudflared)"; return; fi
  if [ -x "$ROOT/bin/cloudflared" ]; then CF_BIN="$ROOT/bin/cloudflared"; return; fi
  log "cloudflared not found - downloading official binary to bin/cloudflared"
  mkdir -p "$ROOT/bin"
  local arch os
  case "$(uname -m)" in
    x86_64) arch=amd64;; aarch64|arm64) arch=arm64;; *) arch=amd64;;
  esac
  os="$(uname -s | tr '[:upper:]' '[:lower:]')"
  curl -fsSL -o "$ROOT/bin/cloudflared" \
    "https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-${os}-${arch}"
  chmod +x "$ROOT/bin/cloudflared"
  CF_BIN="$ROOT/bin/cloudflared"
}

# running returns 0 when the tracked cloudflared pid is alive.
running() {
  [ -f "$PID_FILE" ] || return 1
  local pid; pid="$(cat "$PID_FILE" 2>/dev/null || true)"
  [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null
}

# set_env_var replaces (or appends) KEY=VALUE in .env. Uses a non-/ sed
# delimiter since values are URLs.
set_env_var() {
  local key="$1" val="$2"
  if grep -qE "^${key}=" "$ENV_FILE"; then
    sed -i -E "s#^${key}=.*#${key}=${val}#" "$ENV_FILE"
  else
    printf '%s=%s\n' "$key" "$val" >> "$ENV_FILE"
  fi
}

cmd_up() {
  mkdir -p "$STATE_DIR"
  if running; then
    log "tunnel already running: $(cat "$URL_FILE" 2>/dev/null || echo '?')"
    return 0
  fi
  [ -f "$ENV_FILE" ] || { err ".env not found - run 'make local-env' first"; exit 1; }
  resolve_cloudflared

  log "starting cloudflared quick tunnel -> http://localhost:${PORT}"
  : > "$LOG_FILE"
  nohup "$CF_BIN" tunnel --url "http://localhost:${PORT}" --no-autoupdate >>"$LOG_FILE" 2>&1 &
  echo $! > "$PID_FILE"

  local url=""
  for _ in $(seq 1 30); do
    url="$(grep -oE 'https://[a-z0-9-]+\.trycloudflare\.com' "$LOG_FILE" | head -1 || true)"
    [ -n "$url" ] && break
    sleep 1
  done
  if [ -z "$url" ]; then
    err "tunnel did not come up; last log lines:"; tail -n 15 "$LOG_FILE" >&2
    cmd_down >/dev/null 2>&1 || true
    exit 1
  fi
  echo "$url" > "$URL_FILE"
  log "tunnel URL: $url"

  # Back up the pristine .env once so repeated `up` never clobbers the original.
  [ -f "$ENV_BACKUP" ] || cp "$ENV_FILE" "$ENV_BACKUP"
  set_env_var BASE_URL "$url"
  set_env_var OAUTH_REDIRECT_URL "${url}${CALLBACK_PATH}"
  log ".env updated (BASE_URL + OAUTH_REDIRECT_URL -> tunnel; backup: .env.bak.preTunnel)"

  log "restarting app to load new env..."
  docker compose up -d >/dev/null
  cat <<EOF

  Tunnel is up.

     Public URL:     $url
     OAuth callback: ${url}${CALLBACK_PATH}

  NEXT (one manual step): register the OAuth callback above in the Discord dev
  portal -> your app -> OAuth2 -> Redirects -> Add -> Save. Without it, login
  fails (Discord rejects unlisted redirect URIs).

  Then the remote player joins the Discord server and runs /create-character
  themselves (the link binds to whoever runs it).

  Tear down when done:  make tunnel-down
EOF
}

cmd_down() {
  if running; then
    local pid; pid="$(cat "$PID_FILE")"
    log "stopping cloudflared (pid $pid)"
    kill "$pid" 2>/dev/null || true
  else
    log "no running tunnel (tracked)"
  fi
  rm -f "$PID_FILE" "$URL_FILE"

  if [ -f "$ENV_BACKUP" ]; then
    mv "$ENV_BACKUP" "$ENV_FILE"
    log ".env restored from backup"
    if docker compose ps >/dev/null 2>&1; then
      log "restarting app to load restored env..."
      docker compose up -d >/dev/null || true
    fi
  fi
  log "tunnel down."
}

cmd_status() {
  if running; then
    log "RUNNING - $(cat "$URL_FILE" 2>/dev/null || echo '?')  (pid $(cat "$PID_FILE"))"
  else
    log "not running"
  fi
}

cmd_url() { [ -f "$URL_FILE" ] && cat "$URL_FILE" || true; }

case "${1:-up}" in
  up)     cmd_up;;
  down)   cmd_down;;
  status) cmd_status;;
  url)    cmd_url;;
  *) echo "usage: $0 {up|down|status|url}" >&2; exit 2;;
esac
