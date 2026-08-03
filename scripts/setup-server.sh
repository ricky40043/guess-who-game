#!/usr/bin/env bash
set -Eeuo pipefail

DEPLOY_DIR="${DEPLOY_DIR:-/opt/guess-who-game}"
ENV_FILE="${DEPLOY_ENV_FILE:-$DEPLOY_DIR/.env}"
SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "$SCRIPT_DIR/.." && pwd)"
EXAMPLE_FILE="$REPO_ROOT/.env.server.example"

log() {
  printf '[%s] %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$*"
}

if [ "$(uname -s)" != "Linux" ]; then
  echo "此腳本只供 Linux Server 使用。"
  exit 1
fi

command -v docker >/dev/null 2>&1 || {
  echo "找不到 Docker。Ubuntu / Debian 可執行："
  echo "curl -fsSL https://get.docker.com | sh"
  exit 1
}

docker compose version >/dev/null 2>&1 || {
  echo "找不到 Docker Compose plugin。"
  exit 1
}

if ! docker info >/dev/null 2>&1; then
  echo "目前帳號無法操作 Docker。請將 runner 使用者加入 docker 群組後重新登入："
  echo "sudo usermod -aG docker \$USER"
  exit 1
fi

SUDO=""
if [ ! -w "$(dirname "$DEPLOY_DIR")" ] 2>/dev/null; then
  command -v sudo >/dev/null 2>&1 || {
    echo "需要建立 $DEPLOY_DIR，但目前沒有權限，也找不到 sudo。"
    exit 1
  }
  SUDO="sudo"
fi

log "建立部署設定目錄：$DEPLOY_DIR"
$SUDO mkdir -p "$DEPLOY_DIR"
$SUDO chown "$(id -u):$(id -g)" "$DEPLOY_DIR"

if [ ! -f "$ENV_FILE" ]; then
  cp "$EXAMPLE_FILE" "$ENV_FILE"
  chmod 600 "$ENV_FILE"
  log "已建立 $ENV_FILE"
else
  chmod 600 "$ENV_FILE"
  log "$ENV_FILE 已存在，保留原內容"
fi

cat <<EOF

✅ Server 基本部署目錄已準備完成

設定檔：$ENV_FILE
預設服務位置：http://127.0.0.1:20931
健康檢查：http://127.0.0.1:20931/api/health

接下來：
1. 在 GitHub Repo 安裝 self-hosted runner。
2. runner 必須加上標籤：guess-who-game-deploy
3. Cloudflare Tunnel / Nginx 指向 http://127.0.0.1:20931
4. 合併 PR 後 push main，CI 通過才會自動部署。
EOF
