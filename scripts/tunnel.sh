#!/usr/bin/env bash
# Expose the local DnDnD app (localhost:8080) over a STABLE public URL via an
# ngrok tunnel bound to a reserved domain, so a remote player can reach the web
# character builder and the Discord OAuth login from their own machine.
#
#   scripts/tunnel.sh up      Download ngrok if missing, start the tunnel on the
#                             reserved domain, point .env (BASE_URL +
#                             OAUTH_REDIRECT_URL) at it, and restart the app.
#   scripts/tunnel.sh down    Stop the tunnel, restore .env, restart the app.
#   scripts/tunnel.sh status  Show whether a tunnel is running and its URL.
#   scripts/tunnel.sh url     Print the current tunnel URL (empty if down).
#
# Unlike the old cloudflared quick tunnel, the URL is STABLE across restarts
# (it's your reserved ngrok domain), so the Discord OAuth redirect only has to
# be registered ONCE in the dev portal and never changes again.
#
# ONE-TIME SETUP (do these yourself; the script never sees your token):
#   1. Create a free ngrok account: https://dashboard.ngrok.com/signup
#   2. Claim your free static domain (Dashboard -> Domains -> "New Domain");
#      it looks like  your-name.ngrok-free.app
#   3. Put both in .env (gitignored):
#        NGROK_DOMAIN=your-name.ngrok-free.app
#        NGROK_AUTHTOKEN=<your authtoken from the ngrok dashboard>
#      (Alternatively run `ngrok config add-authtoken <token>` once instead of
#       putting NGROK_AUTHTOKEN in .env.)
#   4. Register ONCE in Discord dev portal -> your app -> OAuth2 -> Redirects:
#        https://your-name.ngrok-free.app/portal/auth/callback
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

PORT="${APP_PORT:-8080}"
STATE_DIR="$ROOT/.tunnel"
PID_FILE="$STATE_DIR/ngrok.pid"
LOG_FILE="$STATE_DIR/ngrok.log"
URL_FILE="$STATE_DIR/url"
ENV_FILE="$ROOT/.env"
ENV_BACKUP="$ROOT/.env.bak.preTunnel"
CALLBACK_PATH="/portal/auth/callback"
# Legacy cloudflared state, retired on the next up/down so the old quick tunnel
# doesn't linger as an orphan after the switch to ngrok.
LEGACY_CF_PID="$STATE_DIR/cloudflared.pid"
NGROK_BIN=""

log() { printf '\033[36m[tunnel]\033[0m %s\n' "$*"; }
err() { printf '\033[31m[tunnel] ERROR:\033[0m %s\n' "$*" >&2; }

# env_value reads a KEY's value from .env (used for config, not secrets we print).
env_value() {
  local key="$1"
  [ -f "$ENV_FILE" ] || return 0
  grep -E "^${key}=" "$ENV_FILE" 2>/dev/null | head -1 | cut -d= -f2- || true
}

# load_ngrok_config resolves NGROK_DOMAIN (required) and exports NGROK_AUTHTOKEN
# (optional) from the environment, falling back to .env. The authtoken is only
# exported for the child ngrok process; it is never printed or written anywhere.
load_ngrok_config() {
  NGROK_DOMAIN="${NGROK_DOMAIN:-$(env_value NGROK_DOMAIN)}"
  if [ -z "${NGROK_DOMAIN:-}" ]; then
    err "NGROK_DOMAIN not set. Add it to .env, e.g.:"
    err "    NGROK_DOMAIN=your-name.ngrok-free.app"
    err "Claim a free static domain at https://dashboard.ngrok.com/domains"
    exit 1
  fi
  local tok; tok="${NGROK_AUTHTOKEN:-$(env_value NGROK_AUTHTOKEN)}"
  [ -n "${tok:-}" ] && export NGROK_AUTHTOKEN="$tok"
}

# resolve_ngrok sets NGROK_BIN, preferring a system install, then bin/, then
# downloading + extracting the official release tarball into bin/.
resolve_ngrok() {
  if command -v ngrok >/dev/null 2>&1; then NGROK_BIN="$(command -v ngrok)"; return; fi
  if [ -x "$ROOT/bin/ngrok" ]; then NGROK_BIN="$ROOT/bin/ngrok"; return; fi
  log "ngrok not found - downloading official binary to bin/ngrok"
  mkdir -p "$ROOT/bin"
  local arch os tgz
  case "$(uname -m)" in
    x86_64) arch=amd64;; aarch64|arm64) arch=arm64;; *) arch=amd64;;
  esac
  os="$(uname -s | tr '[:upper:]' '[:lower:]')"
  tgz="$(mktemp)"
  curl -fsSL -o "$tgz" \
    "https://bin.equinox.io/c/bNyj1mQVY4c/ngrok-v3-stable-${os}-${arch}.tgz"
  tar -xzf "$tgz" -C "$ROOT/bin" ngrok
  rm -f "$tgz"
  chmod +x "$ROOT/bin/ngrok"
  NGROK_BIN="$ROOT/bin/ngrok"
}

# running returns 0 when the tracked ngrok pid is alive.
running() {
  [ -f "$PID_FILE" ] || return 1
  local pid; pid="$(cat "$PID_FILE" 2>/dev/null || true)"
  [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null
}

# retire_legacy_cloudflared stops a leftover cloudflared quick tunnel from the
# pre-ngrok era so it doesn't keep running as an orphan after the switch.
retire_legacy_cloudflared() {
  [ -f "$LEGACY_CF_PID" ] || return 0
  local pid; pid="$(cat "$LEGACY_CF_PID" 2>/dev/null || true)"
  if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then
    log "retiring legacy cloudflared quick tunnel (pid $pid)"
    kill "$pid" 2>/dev/null || true
  fi
  rm -f "$LEGACY_CF_PID" "$STATE_DIR/cloudflared.log"
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
  retire_legacy_cloudflared
  if running; then
    log "tunnel already running: $(cat "$URL_FILE" 2>/dev/null || echo '?')"
    return 0
  fi
  [ -f "$ENV_FILE" ] || { err ".env not found - run 'make local-env' first"; exit 1; }
  load_ngrok_config
  resolve_ngrok

  local url="https://${NGROK_DOMAIN}"
  log "starting ngrok tunnel ${url} -> http://localhost:${PORT}"
  : > "$LOG_FILE"
  nohup "$NGROK_BIN" http "$PORT" \
    --url="$url" --log=stdout --log-format=logfmt \
    >>"$LOG_FILE" 2>&1 &
  echo $! > "$PID_FILE"

  # Wait for ngrok to report the tunnel established (or fail fast on an error).
  local established=""
  for _ in $(seq 1 30); do
    if ! running; then break; fi
    if grep -q 'msg="started tunnel"' "$LOG_FILE" 2>/dev/null; then established=1; break; fi
    if grep -qiE 'lvl=(eror|crit|error)|ERR_NGROK' "$LOG_FILE" 2>/dev/null; then break; fi
    sleep 1
  done
  if [ -z "$established" ]; then
    err "ngrok tunnel did not come up; last log lines:"; tail -n 15 "$LOG_FILE" >&2
    err "Common causes: missing/invalid NGROK_AUTHTOKEN, or the reserved domain"
    err "NGROK_DOMAIN='${NGROK_DOMAIN}' is wrong or already in use by another agent."
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

  Tunnel is up (stable ngrok domain).

     Public URL:     $url
     OAuth callback: ${url}${CALLBACK_PATH}

  Because the domain is reserved, this URL never changes -> the OAuth callback
  only needs to be registered in the Discord dev portal ONCE (you've already
  done it if a previous run worked). No per-run Discord change required.

  The remote player joins the Discord server and runs /create-character
  themselves (the link binds to whoever runs it).

  Tear down when done:  make tunnel-down
EOF
}

cmd_down() {
  retire_legacy_cloudflared
  if running; then
    local pid; pid="$(cat "$PID_FILE")"
    log "stopping ngrok (pid $pid)"
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
