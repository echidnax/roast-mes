# 烘豆機台 MES

模擬咖啡烘豆機的製造執行系統(MES):透過 OPC UA 即時監控機台、從網頁調整目標溫度,並管理烘焙批次工單。
練習「設備通訊 → 後端 API → 資料庫 → 網頁介面」的完整流程。

![機台監控](docs/machines.png)
![工單管理](docs/orders.png)

## 功能

- **機台監控**:豆溫、風門開度、剩餘烘焙時間、目標溫度、是否在線
- **寫入設備**:從 API 修改目標溫度,經 OPC UA 寫回設備
- **工單管理**:建立、派工、開工、回報產量、完工、取消
- **資料保存**:工單存在 PostgreSQL,程式重啟後仍在
- **斷線重連**:OPC UA 連線中斷會自動重試,恢復後畫面自動回復
- **警報**:豆溫過高、機台故障時顯示警報,恢復後自動解除(假設備視窗按 f 可觸發故障)

## 技術

| 層 | 使用 |
|---|---|
| 後端 | Go(標準函式庫 `net/http`)、gopcua(OPC UA Client)、pgx |
| 設備模擬 | Node.js + node-opcua(假的 OPC UA 伺服器) |
| 資料庫 | PostgreSQL 16(Docker) |
| 前端 | Vue 3、TypeScript、Vite |

## 架構

```
OPC UA 伺服器(設備)◀──訂閱/寫入──▶ Go: OPC UA Client ──▶ Hub(過濾抖動、保存最新值)──▶ /api/machines ──▶ Vue 網頁
                                                                                          
Vue 網頁 ──▶ /api/work-orders ──▶ 工單規則(狀態機)──▶ PostgreSQL
```

```
backend/
  cmd/mes/              主程式與 HTTP 路由
  cmd/simdemo/          在終端機觀察模擬器與警報的小工具
  internal/plc/         設備介面、內建模擬器、OPC UA Client、Hub、警報判斷
  internal/workorder/   工單規則與記憶體版儲存
  internal/pgstore/     PostgreSQL 版儲存
opcua-sim/              假的 OPC UA 伺服器(代替實體設備)
frontend/               Vue 網頁
docker-compose.yml      PostgreSQL
```

## 執行方式
### 最快的方式:Docker 一鍵啟動

```bash
docker compose up --build
```

打開 http://localhost:8081 。會同時啟動資料庫、假設備(OPC UA 伺服器)、後端與前端。

需要 Go、Node.js、Docker Desktop。以下為 Windows PowerShell 指令。

```powershell
# 1. 啟動資料庫
docker compose up -d

# 2. 啟動假設備(OPC UA 伺服器)
cd opcua-sim
npm install
node server.js

# 3. 啟動後端(另一個終端機)
cd backend
$env:DATABASE_URL = "postgres://mes:mes@localhost:5432/mes?sslmode=disable"
$env:PLC_MODE = "opcua"
go run ./cmd/mes

# 4. 啟動前端(另一個終端機)
cd frontend
npm install
npm run dev
```

打開 http://localhost:5173 。

- 不設定 `PLC_MODE`:後端改用內建模擬器,不需要步驟 2。
- 不設定 `DATABASE_URL`:工單改存記憶體(重啟後消失),不需要步驟 1。
- 測試:`cd backend && go test ./...`

## API

| 方法 | 路徑 | 說明 |
|---|---|---|
| GET | `/healthz` | 健康檢查 |
| GET | `/api/machines` | 機台最新狀態與是否在線 |
| PUT | `/api/machines/{id}/set-temperature` | 修改目標溫度 `{value}`(150–250),寫入設備 |
| GET | `/api/work-orders` | 工單列表 |
| POST | `/api/work-orders` | 建立工單 `{order_no, product_spec, planned_qty}` |
| GET | `/api/work-orders/{id}` | 單張工單 |
| POST | `/api/work-orders/{id}/transition` | 改狀態 `{to, machine_id?, version}` |
| POST | `/api/work-orders/{id}/report` | 回報產量 `{qty, version}` |

錯誤統一回傳 `{"error": {"code": "...", "message": "..."}}`。

## 設計重點

- **設備介面(`Source`)**:內建模擬器與 OPC UA 共用同一個介面,用環境變數切換,Hub、API、前端都不用改。
- **OPC UA 訂閱**:不輪詢,資料點有變化才會收到通知;連線中斷後每 3 秒自動重連。
- **死區過濾**:感測器數值只有變化超過門檻才算變動,避免雜訊塞滿資料。
- **工單狀態機**:待派工 → 已派工 → 生產中 → 已完工,任何階段可取消;取消後不可復原,以保留紀錄。規則獨立成純邏輯,有單元測試。
- **樂觀鎖**:每張工單有版本號,更新時版本對不上就回 409,避免兩個人互相覆蓋。
- **資料庫層面的保險**:同一台機器同時只能有一張生產中的工單(唯一索引),不只靠程式判斷。

## 目前限制

- 設備端使用模擬的 OPC UA 伺服器,尚未接過真實 PLC
- OPC UA 連線未啟用加密與帳號驗證
- 警報已顯示在網頁,但紀錄存在記憶體,後端重啟後會清空
- 機台畫面每 2 秒輪詢一次,尚未改為後端主動推送
- 尚無登入與權限