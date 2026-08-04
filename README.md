# 🕵️ 猜人 Guess Who Game

多人即時匿名問答派對遊戲。玩家先回答生活化問題，系統把每個人的整組答案匿名成「同學 A、同學 B……」，最後所有人進行一對一配對猜人，猜對最多者獲勝。

## 已完成功能

- 6 碼房間代碼與多人 WebSocket 即時同步
- 加入房間強制填寫暱稱，暱稱不可重複
- 遊戲開始後鎖定玩家名單，禁止中途加入
- 內建 36 題已審核的生活化題庫
- 隨機抽題、自選題庫、自訂題目
- 預設 5 題，每題 60 秒；房主可設定 15～300 秒
- 全員提交後提前進入下一題，逾時則自動換題
- 固定匿名映射與逐人答案公布
- 手機版一對一配對猜人，每個名字只能使用一次
- 自己的匿名代號與名字自動排除
- 猜對 1 人得 1 分，全對額外加 2 分
- 房主與玩家斷線重連、遊戲狀態恢復
- Docker Compose 與 GitHub Actions CI/CD
- self-hosted runner 自動部署、健康檢查與失敗回滾

## 遊戲流程

```text
等待加入 → 逐題回答 → 匿名公布 → 配對猜人 → 排行榜
```

房主畫面只負責設定與控制，不算玩家。房主要參加時，請另外用手機輸入房號與暱稱加入。

## 本機執行

需要 Go 1.23 以上版本：

```bash
go run .
```

開啟：

```text
http://localhost:8080
```

同一個區域網路內的手機可以使用 Mac 的內網 IP：

```text
http://<Mac 的 IP>:8080
```

## Docker

本機直接執行：

```bash
docker compose up -d --build
```

正式 Server 預設只綁定本機介面：

```text
http://127.0.0.1:20931
```

供 Cloudflare Tunnel 或 Nginx 反向代理使用。

## 自動 CI/CD

`main` 有新 Commit 時：

```text
GitHub 雲端測試
  → gofmt
  → go test -race
  → go vet
  → Go production build
  → Docker build
  → 地端 self-hosted runner
  → Docker Compose 更新
  → 健康檢查
  → 失敗自動回滾上一版
```

Server 第一次準備：

```bash
bash scripts/setup-server.sh
```

完整設定請看：

- [地端自動部署指南](docs/地端自動部署指南.md)
- [架構說明](docs/ARCHITECTURE.md)

## 技術架構

- 後端：Go `net/http`
- 即時連線：專案內建 RFC 6455 WebSocket 實作，無第三方執行依賴
- 前端：原生 JavaScript、HTML、CSS，直接嵌入 Go 執行檔
- 資料：房間、答案與結果只存在記憶體，服務重啟後清除
- 部署：Docker Compose + GitHub-hosted CI + self-hosted deploy runner

## WebSocket 主要事件

Client → Server：

```text
CREATE_ROOM
JOIN_ROOM
REJOIN_ROOM
UPDATE_SETTINGS
START_GAME
SUBMIT_ANSWER
NEXT_REVEAL
START_GUESSING
SUBMIT_GUESSES
PING
```

Server → Client：

```text
ROOM_CREATED
JOIN_SUCCESS
REJOIN_SUCCESS
QUESTION_STARTED
ANSWER_PROGRESS
REVEAL_STARTED
PROFILE_REVEALED
REVEAL_COMPLETE
GUESSING_STARTED
GUESS_PROGRESS
GAME_FINISHED
ERROR
```

## 第一版限制

- 所有資料都在單一服務記憶體，尚未支援多台後端水平擴充。
- 沒有永久保存歷史戰績。
- 房間尚未實作閒置自動清理，正式長期對外服務前應補上。
