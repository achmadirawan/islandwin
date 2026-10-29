param (
    [string]$Target = "release"
)

$ErrorActionPreference = "Stop"

switch ($Target) {
    "release" {
        Write-Host "Building IslandWin (Release, windowsgui)..." -ForegroundColor Cyan
        go build -ldflags="-s -w -H=windowsgui" -o bin\islandwin.exe .\cmd\islandwin
        $item = Get-Item bin\islandwin.exe
        Write-Host ("Build complete: bin\islandwin.exe ({0:N2} MB)" -f ($item.Length / 1MB)) -ForegroundColor Green
    }
    "debug" {
        Write-Host "Building IslandWin (Debug, console)..." -ForegroundColor Cyan
        go build -o bin\islandwin-debug.exe .\cmd\islandwin
        $item = Get-Item bin\islandwin-debug.exe
        Write-Host ("Build complete: bin\islandwin-debug.exe ({0:N2} MB)" -f ($item.Length / 1MB)) -ForegroundColor Green
    }
    "test" {
        Write-Host "Running all tests..." -ForegroundColor Cyan
        go test -v ./...
    }
    default {
        Write-Host "Usage: .\scripts\build.ps1 -Target [release|debug|test]" -ForegroundColor Yellow
    }
}
