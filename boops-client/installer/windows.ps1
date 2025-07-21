New-Service -Name "BoopsClient" `
  -BinaryPathName "C:\Path\To\boops.exe sync" `
  -DisplayName "Boops Client Sync Service" `
  -StartupType Automatic
Start-Service -Name "BoopsClient"

