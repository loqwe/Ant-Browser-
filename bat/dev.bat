@echo off
setlocal EnableExtensions EnableDelayedExpansion

cd /d "%~dp0.."

echo ========================================
echo   Ant Chrome - Dev Launcher
echo ========================================
echo.
echo Current workdir: %CD%
echo.

call :cleanup_dev_logs

set PREFERRED_FRONTEND_PORT=5218
set FRONTEND_PORT=

if not defined FRONTEND_NODE_MAX_OLD_SPACE_SIZE_MB set FRONTEND_NODE_MAX_OLD_SPACE_SIZE_MB=256
if not defined FRONTEND_NODE_MAX_SEMI_SPACE_SIZE_MB set FRONTEND_NODE_MAX_SEMI_SPACE_SIZE_MB=16
if not defined FRONTEND_NODE_RSS_WARN_MB set FRONTEND_NODE_RSS_WARN_MB=256
if not defined FRONTEND_NODE_RSS_HARD_LIMIT_MB set FRONTEND_NODE_RSS_HARD_LIMIT_MB=360
if not defined FRONTEND_NODE_MEMORY_POLL_MS set FRONTEND_NODE_MEMORY_POLL_MS=3000

echo Cleaning stale processes...
node frontend\scripts\dev-port-helper.mjs cleanup
if errorlevel 1 (
    echo [ERROR] Failed to clean stale frontend dev processes.
    pause
    exit /b 1
)
taskkill /F /IM ant-chrome-dev.exe >nul 2>&1
taskkill /F /IM ant-chrome.exe >nul 2>&1
echo.

echo Resolving frontend dev port...
for /f "usebackq delims=" %%a in (`node frontend\scripts\dev-port-helper.mjs resolve --preferred %PREFERRED_FRONTEND_PORT%`) do (
    if not defined FRONTEND_PORT set "FRONTEND_PORT=%%a"
)
if not defined FRONTEND_PORT (
    echo [ERROR] Failed to resolve frontend dev port.
    pause
    exit /b 1
)
if not "%FRONTEND_PORT%"=="%PREFERRED_FRONTEND_PORT%" (
    echo [ERROR] Preferred frontend port %PREFERRED_FRONTEND_PORT% is occupied by another program.
    echo         Wails dev in current mode must use the fixed port %PREFERRED_FRONTEND_PORT%.
    echo         Please free that port and retry.
    pause
    exit /b 1
)
echo [OK] Frontend dev port: %FRONTEND_PORT%
echo.
set FRONTEND_PORT=%PREFERRED_FRONTEND_PORT%
echo Frontend Node old-space limit: %FRONTEND_NODE_MAX_OLD_SPACE_SIZE_MB% MB
echo Frontend Node semi-space limit: %FRONTEND_NODE_MAX_SEMI_SPACE_SIZE_MB% MB
echo Frontend Node RSS warning: %FRONTEND_NODE_RSS_WARN_MB% MB
echo Frontend Node RSS hard limit: %FRONTEND_NODE_RSS_HARD_LIMIT_MB% MB
echo Frontend Node RSS poll interval: %FRONTEND_NODE_MEMORY_POLL_MS% ms
echo.

set GOPROXY=https://goproxy.cn,direct

echo Checking dependencies...
if not exist "go.mod" (
    echo [ERROR] go.mod not found in repository root.
    echo         This development branch must keep a complete Go source tree.
    pause
    exit /b 1
)
if not exist "wails.json" (
    echo [ERROR] wails.json not found in repository root.
    echo         This development branch must keep a complete Wails source tree.
    pause
    exit /b 1
)
echo Installing Go dependencies...
go mod download
go mod tidy
if errorlevel 1 (
    echo [ERROR] Failed to install Go dependencies.
    pause
    exit /b 1
)

if not exist "frontend\node_modules" (
    echo Installing frontend dependencies...
    pushd frontend
    call npm install
    popd
)
echo.

echo Regenerating Wails bindings...
call bat\generate-bindings.bat --no-pause
if errorlevel 1 (
    echo [ERROR] Failed to generate Wails bindings.
    pause
    exit /b 1
)
if not exist "frontend\src\wailsjs" (
    echo [ERROR] Wails bindings output folder not found.
    pause
    exit /b 1
)
echo.

echo Starting Wails dev...
echo Frontend URL: http://127.0.0.1:%FRONTEND_PORT%
echo Wails dev endpoint: http://127.0.0.1:%FRONTEND_PORT%
echo.

wails dev -s -viteservertimeout 60
set EXIT_CODE=%errorlevel%

if not "%EXIT_CODE%"=="0" (
    echo.
    echo [ERROR] wails dev exited with code %EXIT_CODE%.
)

pause
exit /b %EXIT_CODE%

:cleanup_dev_logs
for %%f in (
    "tmp-npm-dev.err.log"
    "tmp-npm-dev.log"
    "tmp-wails-err.log"
    "tmp-wails-out.log"
    "tmp-wails2-err.log"
    "tmp-wails2-out.log"
    "tmp-wails3-err.log"
    "tmp-wails3-out.log"
    "tmp-wails.err"
    "wails-dev-capture.log"
    "wails-dev-run.log"
    "wails-dev-stderr.log"
    "wails-dev-stdout.log"
) do (
    if exist %%~f del /F /Q %%~f >nul 2>&1
)
exit /b 0
