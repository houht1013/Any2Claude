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

:: Navigate to project root (script is in scripts/)
cd /d "%~dp0.."
set PROJECT_ROOT=%CD%
set BUILD_PKG=./cmd/any2claude

if "%1"=="all" goto :build_all
if "%1"=="mac" goto :build_mac
if "%1"=="linux" goto :build_linux

:build_windows
echo [1/1] Building Windows exe...
go build -ldflags="-s -w -H windowsgui" -o Any2Claude.exe %BUILD_PKG%
if errorlevel 1 (
    echo [ERROR] Build failed.
    pause
    exit /b 1
)
echo.
echo [OK] Build success!
for %%A in (Any2Claude.exe) do echo   Output: %PROJECT_ROOT%\%%A  (%%~zA bytes)
echo.
echo   Usage: Double-click Any2Claude.exe
echo          Right-click tray icon to open dashboard or quit.
echo.
echo   Tip: Run "scripts\build.bat all" to cross-compile for all platforms.
echo.
pause
exit /b 0

:build_mac
echo [1/2] Building macOS (Intel amd64)...
set GOOS=darwin
set GOARCH=amd64
go build -ldflags="-s -w" -o Any2Claude-darwin-amd64 %BUILD_PKG%
if errorlevel 1 ( echo [ERROR] macOS amd64 build failed. & pause & exit /b 1 )
for %%A in (Any2Claude-darwin-amd64) do echo   OK: %%A  (%%~zA bytes)

echo [2/2] Building macOS (Apple Silicon arm64)...
set GOOS=darwin
set GOARCH=arm64
go build -ldflags="-s -w" -o Any2Claude-darwin-arm64 %BUILD_PKG%
if errorlevel 1 ( echo [ERROR] macOS arm64 build failed. & pause & exit /b 1 )
for %%A in (Any2Claude-darwin-arm64) do echo   OK: %%A  (%%~zA bytes)

set GOOS=windows
set GOARCH=amd64
echo.
echo [OK] macOS builds complete!
echo   Transfer to Mac, then: chmod +x Any2Claude-darwin-* ^&^& ./Any2Claude-darwin-arm64
echo.
pause
exit /b 0

:build_linux
echo [1/1] Building Linux (amd64)...
set GOOS=linux
set GOARCH=amd64
go build -ldflags="-s -w" -o Any2Claude-linux-amd64 %BUILD_PKG%
if errorlevel 1 ( echo [ERROR] Linux build failed. & pause & exit /b 1 )
for %%A in (Any2Claude-linux-amd64) do echo   OK: %%A  (%%~zA bytes)

set GOOS=windows
set GOARCH=amd64
echo.
echo [OK] Linux build complete!
echo   Transfer to server, then: chmod +x Any2Claude-linux-amd64 ^&^& ./Any2Claude-linux-amd64 -host 0.0.0.0
echo.
pause
exit /b 0

:build_all
echo Building all platforms...
echo.

echo [1/4] Windows (amd64)...
set GOOS=windows
set GOARCH=amd64
go build -ldflags="-s -w -H windowsgui" -o Any2Claude.exe %BUILD_PKG%
if errorlevel 1 ( echo [ERROR] Windows build failed. & pause & exit /b 1 )
for %%A in (Any2Claude.exe) do echo   OK: %%A  (%%~zA bytes)

echo [2/4] macOS (Intel amd64)...
set GOOS=darwin
set GOARCH=amd64
go build -ldflags="-s -w" -o Any2Claude-darwin-amd64 %BUILD_PKG%
if errorlevel 1 ( echo [ERROR] macOS amd64 build failed. & pause & exit /b 1 )
for %%A in (Any2Claude-darwin-amd64) do echo   OK: %%A  (%%~zA bytes)

echo [3/4] macOS (Apple Silicon arm64)...
set GOOS=darwin
set GOARCH=arm64
go build -ldflags="-s -w" -o Any2Claude-darwin-arm64 %BUILD_PKG%
if errorlevel 1 ( echo [ERROR] macOS arm64 build failed. & pause & exit /b 1 )
for %%A in (Any2Claude-darwin-arm64) do echo   OK: %%A  (%%~zA bytes)

echo [4/4] Linux (amd64)...
set GOOS=linux
set GOARCH=amd64
go build -ldflags="-s -w" -o Any2Claude-linux-amd64 %BUILD_PKG%
if errorlevel 1 ( echo [ERROR] Linux build failed. & pause & exit /b 1 )
for %%A in (Any2Claude-linux-amd64) do echo   OK: %%A  (%%~zA bytes)

set GOOS=windows
set GOARCH=amd64

echo.
echo ============================================================
echo   All platforms built successfully!
echo ============================================================
echo.
echo   Windows:        Any2Claude.exe
echo   macOS Intel:    Any2Claude-darwin-amd64
echo   macOS ARM:      Any2Claude-darwin-arm64
echo   Linux:          Any2Claude-linux-amd64
echo.
echo   macOS/Linux: chmod +x Any2Claude-* then run directly.
echo   No Go needed on the target machine.
echo.
pause
exit /b 0
