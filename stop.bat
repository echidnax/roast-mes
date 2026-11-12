@echo off
chcp 65001 >nul
cd /d "%~dp0"

echo ===== 關閉前端、後端、假設備 =====
taskkill /FI "WINDOWTITLE eq Frontend*" /T /F >nul 2>&1
taskkill /FI "WINDOWTITLE eq Backend*" /T /F >nul 2>&1
taskkill /FI "WINDOWTITLE eq PLC*" /T /F >nul 2>&1

rem 保險:再依連接埠,把還佔著 5173(前端)、8080(後端)、4840(假設備)的程式關掉
powershell -NoProfile -Command "foreach ($p in 5173,8080,4840) { Get-NetTCPConnection -LocalPort $p -State Listen -ErrorAction SilentlyContinue | ForEach-Object { Stop-Process -Id $_.OwningProcess -Force -ErrorAction SilentlyContinue } }"

echo.
echo ===== 停止資料庫 =====
docker compose stop

echo.
echo 全部已停止。資料有保留,下次雙擊 start.bat 就會回來。
pause