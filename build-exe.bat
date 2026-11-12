@echo off
REM Builds rdio-scanner_v5.1.exe. Requires Go (https://go.dev/dl/) and Node.js LTS (https://nodejs.org/).
setlocal
cd /d "%~dp0"

where go >nul 2>nul || (echo Go is not installed or not on PATH. & goto :fail)
where node >nul 2>nul || (echo Node.js is not installed or not on PATH. & goto :fail)

echo === Building web client ===
cd client
call npm ci --no-audit --no-fund || goto :fail
set NODE_OPTIONS=--openssl-legacy-provider
call npm run build || goto :fail
cd ..

echo === Building Windows exe ===
cd server
set GOOS=windows
set GOARCH=amd64
set CGO_ENABLED=0
go build -o ..\rdio-scanner_v5.1.exe . || goto :fail
cd ..

echo.
echo Built: %~dp0rdio-scanner_v5.1.exe
pause
exit /b 0

:fail
echo.
echo BUILD FAILED
pause
exit /b 1
