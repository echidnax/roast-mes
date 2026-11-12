@echo off
chcp 65001 >nul
cd /d "%~dp0"
title 烘豆機台 MES 啟動器

echo ===== 1/4 檢查 Docker =====
set TRIES=0
docker info >nul 2>&1
if not errorlevel 1 goto docker_ready

echo Docker 還沒啟動,正在開啟 Docker Desktop...
start "" "C:\Program Files\Docker\Docker\Docker Desktop.exe"

:wait_docker
set /a TRIES+=1
if %TRIES% gtr 40 goto docker_fail
echo 等待 Docker 啟動中... (%TRIES%/40)
timeout /t 5 /nobreak >nul
docker info >nul 2>&1
if errorlevel 1 goto wait_docker

:docker_ready
echo Docker 已就緒

echo.
echo ===== 2/4 啟動資料庫 =====
docker compose up -d
if errorlevel 1 goto docker_fail

set TRIES=0
:wait_db
set /a TRIES+=1
if %TRIES% gtr 30 goto db_fail
docker compose exec -T db pg_isready -U mes -d mes >nul 2>&1
if not errorlevel 1 goto db_ready
echo 等待資料庫就緒... (%TRIES%/30)
timeout /t 2 /nobreak >nul
goto wait_db

:db_ready
echo 資料庫已就緒

echo.
echo ===== 3/4 啟動假設備與後端 =====
start "PLC" cmd /k "cd /d %~dp0opcua-sim && node server.js"
timeout /t 3 /nobreak >nul
start "Backend" cmd /k "cd /d %~dp0backend && set DATABASE_URL=postgres://mes:mes@localhost:5432/mes?sslmode=disable&& set PLC_MODE=opcua&& go run ./cmd/mes"

echo.
echo ===== 4/4 啟動前端 =====
start "Frontend" cmd /k "cd /d %~dp0frontend && npm run dev"

echo.
echo 等待服務啟動後自動開啟瀏覽器...
timeout /t 12 /nobreak >nul
start http://localhost:5173

echo.
echo 全部啟動完成。三個黑色視窗(PLC、Backend、Frontend)要保持開著。
echo 如果網頁顯示紅色錯誤,等幾秒按 F5 重新整理。
echo 這個視窗可以關掉。
pause
exit /b 0

:docker_fail
echo.
echo [失敗] Docker 沒有成功啟動。請手動打開 Docker Desktop,等左下角顯示 Engine running 後再雙擊一次。
pause
exit /b 1

:db_fail
echo.
echo [失敗] 資料庫沒有就緒。請在這個資料夾執行 docker compose logs db 查看原因。
pause
exit /b 1