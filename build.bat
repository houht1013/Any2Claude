@echo off
chcp 65001 >nul 2>&1

echo.
echo ============================================================
echo   Any2Claude (Go) - Build (zero dependencies)
echo ============================================================
echo.

go version >nul 2>&1
if errorlevel 1 (
    echo [ERROR] Go not found.
    echo         Download from: https://go.dev/dl/
    echo         Install the .msi file, then re-run this script.
    pause
    exit /b 1
)
echo [OK] Go found

echo.
echo Building executable (no network required)...
go build -ldflags="-s -w -H windowsgui" -o Any2Claude.exe .
if errorlevel 1 (
    echo [ERROR] Build failed. Check errors above.
    pause
    exit /b 1
)

echo.
echo [OK] Build success!
echo.
for %%A in (Any2Claude.exe) do echo   Output: %CD%\%%A  (%%~zA bytes)
echo.
echo   Usage: Double-click Any2Claude.exe
echo          Right-click tray icon to open dashboard or quit.
echo.
pause
