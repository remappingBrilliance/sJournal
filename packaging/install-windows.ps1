# Installs sJournal for the current user (no admin rights needed) and adds a
# Start Menu entry. Run from the folder produced by build-windows.sh:
#
#     powershell -ExecutionPolicy Bypass -File .\install.ps1
#     powershell -ExecutionPolicy Bypass -File .\install.ps1 -Desktop
#     powershell -ExecutionPolicy Bypass -File .\install.ps1 -NotesDir 'D:\Journal'
#
[CmdletBinding()]
param(
    [switch]$Desktop,
    [string]$NotesDir
)

$ErrorActionPreference = 'Stop'
$src = Split-Path -Parent $MyInvocation.MyCommand.Path

$installDir = Join-Path $env:LOCALAPPDATA 'Programs\sJournal'
$startMenu  = Join-Path $env:APPDATA 'Microsoft\Windows\Start Menu\Programs'
$exe        = Join-Path $installDir 'sjournal.exe'
$ico        = Join-Path $installDir 'sjournal.ico'

Write-Host ">> installing to $installDir"
New-Item -ItemType Directory -Force -Path $installDir | Out-Null
Copy-Item (Join-Path $src 'sjournal.exe') $exe -Force
if (Test-Path (Join-Path $src 'sjournal.ico')) {
    Copy-Item (Join-Path $src 'sjournal.ico') $ico -Force
} else {
    $ico = $exe
}

# Seed config.yaml next to the exe on first install (portable-folder model).
$cfg = Join-Path $installDir 'config.yaml'
if (-not (Test-Path $cfg)) {
    if (-not $NotesDir) { $NotesDir = Join-Path ([Environment]::GetFolderPath('MyDocuments')) 'sJournal' }
    New-Item -ItemType Directory -Force -Path $NotesDir | Out-Null
    @"
remind_me:
    start_time: "09:00"
    end_time: "17:00"
    interval_minutes: 60
    enabled: false
timestamp_default: false
notes_folder: $NotesDir
single_instance: true
require_password_for_edit: false
minimize_to_tray: false
"@ | Set-Content -Path $cfg -Encoding UTF8
    Write-Host ">> journal files will be stored in $NotesDir"
}

function New-Shortcut($linkPath) {
    $ws = New-Object -ComObject WScript.Shell
    $sc = $ws.CreateShortcut($linkPath)
    $sc.TargetPath       = $exe
    $sc.WorkingDirectory = $installDir
    $sc.IconLocation     = $ico
    $sc.Description       = 'A secure, portable journal'
    $sc.Save()
}

New-Item -ItemType Directory -Force -Path $startMenu | Out-Null
New-Shortcut (Join-Path $startMenu 'sJournal.lnk')
if ($Desktop) {
    New-Shortcut (Join-Path ([Environment]::GetFolderPath('Desktop')) 'sJournal.lnk')
}

Write-Host ""
Write-Host "sJournal installed. Find it in the Start Menu."
Write-Host "Uninstall with:  powershell -ExecutionPolicy Bypass -File `"$installDir\uninstall.ps1`""
Copy-Item (Join-Path $src 'uninstall.ps1') (Join-Path $installDir 'uninstall.ps1') -Force
