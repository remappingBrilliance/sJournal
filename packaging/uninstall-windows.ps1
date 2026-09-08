# Removes the per-user sJournal install and its shortcuts. Journal data
# (your notes folder) is left untouched.
#
#     powershell -ExecutionPolicy Bypass -File .\uninstall.ps1
#
$ErrorActionPreference = 'SilentlyContinue'

$installDir = Join-Path $env:LOCALAPPDATA 'Programs\sJournal'
$startMenu  = Join-Path $env:APPDATA 'Microsoft\Windows\Start Menu\Programs\sJournal.lnk'
$desktop    = Join-Path ([Environment]::GetFolderPath('Desktop')) 'sJournal.lnk'

Remove-Item $startMenu -Force
Remove-Item $desktop -Force

# Keep config.yaml + notes; drop the program files.
Remove-Item (Join-Path $installDir 'sjournal.exe') -Force
Remove-Item (Join-Path $installDir 'sjournal.ico') -Force
Remove-Item (Join-Path $installDir 'uninstall.ps1') -Force

Write-Host "sJournal uninstalled."
Write-Host "Kept: $installDir\config.yaml and your notes folder."
Write-Host "Delete $installDir by hand if you want it fully gone."
