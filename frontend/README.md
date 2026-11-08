# 烘豆機台 MES

模擬咖啡烘豆機的製造執行系統(MES):即時監控機台狀態,並管理烘焙批次工單。
用來練習「設備資料收集 → 後端 API → 資料庫 → 網頁介面」的完整流程。

![機台監控](docs/machines.png)
![工單管理](docs/orders.png)

## 功能

- **機台監控**:顯示豆溫、風門開度、剩餘烘焙時間、目標溫度,以及是否在線
- **工單管理**:建立、派工、開工、回報產量、完工、取消
- **資料保存**:工單存在 PostgreSQL,程式重啟後仍在

## 技術

| 層 | 使用 |
|---|---|
| 後端 | Go(標準函式庫 `net/http`)、pgx |
| 資料庫 | PostgreSQL 16(Docker) |
| 前端 | Vue 3、TypeScript、Vite |

## 架構

```
模擬設備 ──▶ Hub(過濾抖動、保存最新值)──▶ GET /api/machines ──▶ Vue 網頁
Vue 網頁 ──▶ /api/work-orders ──▶ 工單規則(狀態機)──▶ PostgreSQL
```

```
backend/
  cmd/mes/              主程式與 HTTP 路由
  cmd/simdemo/          在終端機觀察模擬器與警報的小工具
  internal/plc/         設備介面、模擬器、Hub、警報判斷
  internal/workorder/   工單規則與記憶體版儲存
  internal/pgstore/     PostgreSQL 版儲存
frontend/               Vue 網頁
docker-compose.yml      PostgreSQL
```

## 執行方式

需要 Go、Node.js、Docker Desktop。

```bash
# 1. 啟動資料庫
docker compose up -d

# 2. 啟動後端(PowerShell)
cd backend
$env:DATABASE_URL = "postgres://mes:mes@localhost:5432/mes?sslmode=disable"
go run ./cmd/mes

# 3. 啟動前端(另一個終端機)
cd frontend
npm install
npm run dev
```

打開 http://localhost:5173 。沒有設定 `DATABASE_URL` 時,後端會改用記憶體儲存(重啟後資料消失)。

測試:`cd backend && go test ./...`

## API

| 方法 | 路徑 | 說明 |
|---|---|---|
| GET | `/healthz` | 健康檢查 |
| GET | `/api/machines` | 機台最新狀態與是否在線 |
| GET | `/api/work-orders` | 工單列表 |
| POST | `/api/work-orders` | 建立工單 `{order_no, product_spec, planned_qty}` |
| GET | `/api/work-orders/{id}` | 單張工單 |
| POST | `/api/work-orders/{id}/transition` | 改狀態 `{to, machine_id?, version}` |
| POST | `/api/work-orders/{id}/report` | 回報產量 `{qty, version}` |

錯誤統一回傳 `{"error": {"code": "...", "message": "..."}}`。

## 設計重點

- **設備介面(`Source`)**:模擬器與未來的真實設備共用同一個介面,沒有實體設備也能開發與測試。
- **死區過濾**:感測器數值只有變化超過門檻才算變動,避免雜訊塞滿資料。
- **工單狀態機**:待派工 → 已派工 → 生產中 → 已完工,任何階段可取消;取消後不可復原,以保留紀錄。規則獨立成純邏輯,有單元測試。
- **樂觀鎖**:每張工單有版本號,更新時版本對不上就回 409,避免兩個人互相覆蓋。
- **資料庫層面的保險**:同一台機器同時只能有一張生產中的工單(唯一索引),不只靠程式判斷。

## 目前限制與之後想做的

- 設備端目前只有模擬器,尚未實作 OPC UA 連線
- 警報判斷(超溫、故障)已完成並有測試,但還沒接到網頁
- 機台畫面是每 2 秒輪詢,之後可改成後端主動推送
- 尚無登入與權限