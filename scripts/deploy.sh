#!/usr/bin/env bash
set -Eeuo pipefail

ENV_FILE="${DEPLOY_ENV_FILE:-/opt/guess-who-game/.env}"
PROJECT_NAME="guess-who-game"
IMAGE_NAME="guess-who-game:latest"
ROLLBACK_IMAGE="guess-who-game:rollback"
CONTAINER_NAME="guess-who-game"
HEALTH_URL="http://localhost:8080/api/health"
LOCK_FILE="/tmp/guess-who-game-deploy.lock"

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

# 防止手動部署與 GitHub Actions 同時執行。
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
      docker exec "$CONTAINER_NAME" wget -qO- "$HEALTH_URL" || true
      return 0
    fi
    log "等待服務啟動… ($i/$attempts)"
    sleep "$delay"
  done
  return 1
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

log "驗證 Compose 設定"
"${COMPOSE[@]}" config --quiet

had_previous=false
if docker image inspect "$IMAGE_NAME" >/dev/null 2>&1; then
  log "保存目前 image 作為回滾版本"
  docker tag "$IMAGE_NAME" "$ROLLBACK_IMAGE"
  had_previous=true
fi

log "建置新版 image"
"${COMPOSE[@]}" build --pull app

log "更新服務"
"${COMPOSE[@]}" up -d --remove-orphans app

if ! health_check 30 2; then
  log "新版服務未在 60 秒內通過健康檢查"
  "${COMPOSE[@]}" ps || true
  "${COMPOSE[@]}" logs --tail 120 app || true

  if [ "$had_previous" = true ]; then
    rollback || true
  fi
  exit 1
fi

log "清理未使用的舊 image"
docker image prune -f >/dev/null

log "部署完成"
"${COMPOSE[@]}" ps
