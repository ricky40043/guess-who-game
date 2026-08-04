#!/usr/bin/env bash
set -Eeuo pipefail

ENV_FILE="${DEPLOY_ENV_FILE:-/opt/guess-who-game/.env}"
PROJECT_NAME="guess-who-game"
IMAGE_NAME="guess-who-game:latest"
ROLLBACK_IMAGE="guess-who-game:rollback"
CONTAINER_NAME="guess-who-game"
HEALTH_URL="http://localhost:8080/api/health"
VERSION_URL="http://localhost:8080/api/version"
PUBLIC_URL="${PUBLIC_URL:-https://guess-who.ricky-nova.com}"
BUILD_VERSION="${GITHUB_SHA:-$(git rev-parse HEAD 2>/dev/null || echo dev)}"
LOCK_FILE="/tmp/guess-who-game-deploy.lock"

export BUILD_VERSION

log() {
  printf '[%s] %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$*"
}

fail() {
  log "❌ $*"
  exit 1
}

command -v docker >/dev/null 2>&1 || fail "找不到 docker"
docker compose version >/dev/null 2>&1 || fail "找不到 docker compose plugin"
[ -f "$ENV_FILE" ] || fail "找不到部署設定檔：$ENV_FILE"

if command -v flock >/dev/null 2>&1; then
  exec 9>"$LOCK_FILE"
  flock -n 9 || fail "已有另一個部署正在執行"
fi

COMPOSE=(docker compose --project-name "$PROJECT_NAME" --env-file "$ENV_FILE")

health_check() {
  local attempts="${1:-30}"
  local delay="${2:-2}"
  local i

  for i in $(seq 1 "$attempts"); do
    if docker exec "$CONTAINER_NAME" wget -qO- "$HEALTH_URL" >/dev/null 2>&1; then
      log "✅ 健康檢查通過"
      return 0
    fi
    log "等待服務啟動… ($i/$attempts)"
    sleep "$delay"
  done
  return 1
}

verify_local_version() {
  local response
  response=$(docker exec "$CONTAINER_NAME" wget -qO- "$VERSION_URL") || return 1
  log "容器版本：$response"
  printf '%s' "$response" | grep -Fq "$BUILD_VERSION"
}

verify_public_release() {
  local version_response
  local app_js

  command -v curl >/dev/null 2>&1 || {
    log "Server 找不到 curl，跳過公開網址驗證"
    return 0
  }

  version_response=$(curl -fsS --max-time 20 -H 'Cache-Control: no-cache' "$PUBLIC_URL/api/version?ts=$(date +%s)") || return 1
  log "公開網址版本：$version_response"
  printf '%s' "$version_response" | grep -Fq "$BUILD_VERSION" || return 1

  app_js=$(curl -fsS --max-time 20 -H 'Cache-Control: no-cache' "$PUBLIC_URL/app.js?ts=$(date +%s)") || return 1
  printf '%s' "$app_js" | grep -Fq '掃描 QR Code 加入' || return 1
  printf '%s' "$app_js" | grep -Fq 'api.qrserver.com' || return 1

  log "✅ 公開網址已確認為本次版本，且包含 QR Code 功能"
}

rollback() {
  if ! docker image inspect "$ROLLBACK_IMAGE" >/dev/null 2>&1; then
    log "沒有可用的上一版 image，無法自動回滾"
    return 1
  fi

  log "開始回滾上一版 image"
  docker tag "$ROLLBACK_IMAGE" "$IMAGE_NAME"
  "${COMPOSE[@]}" up -d --no-build --force-recreate --remove-orphans app

  if health_check 20 2; then
    log "✅ 已回滾到上一版"
    return 0
  fi

  log "❌ 回滾後仍無法通過健康檢查"
  return 1
}

log "本次部署版本：$BUILD_VERSION"
log "驗證 Compose 設定"
"${COMPOSE[@]}" config --quiet

had_previous=false
if docker image inspect "$IMAGE_NAME" >/dev/null 2>&1; then
  log "保存目前 image 作為回滾版本"
  docker tag "$IMAGE_NAME" "$ROLLBACK_IMAGE"
  had_previous=true
fi

log "建置新版 image"
"${COMPOSE[@]}" build --pull --no-cache app

log "更新服務"
"${COMPOSE[@]}" up -d --force-recreate --remove-orphans app

if ! health_check 30 2 || ! verify_local_version; then
  log "新版服務健康或版本驗證失敗"
  "${COMPOSE[@]}" ps || true
  "${COMPOSE[@]}" logs --tail 120 app || true

  if [ "$had_previous" = true ]; then
    rollback || true
  fi
  exit 1
fi

if ! verify_public_release; then
  log "公開網址尚未指向本次部署版本，或公開 app.js 不含 QR Code"
  log "請檢查 Cloudflare Tunnel upstream、反向代理或是否有另一個舊容器佔用服務"
  "${COMPOSE[@]}" ps || true
  exit 1
fi

log "清理未使用的舊 image"
docker image prune -f >/dev/null

log "部署完成"
"${COMPOSE[@]}" ps
