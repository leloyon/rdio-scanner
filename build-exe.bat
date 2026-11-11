@echo off
REM Requires Go (https://go.dev/dl/). The webapp is already built in server\webapp.
cd /d "%~dp0server"
set GOOS=windows
set GOARCH=amd64
set CGO_ENABLED=0
go build -o ..\rdio-scanner_v5.1.exe .
if errorlevel 1 (echo BUILD FAILED) else (echo Built: %~dp0rdio-scanner_v5.1.exe)
pause
