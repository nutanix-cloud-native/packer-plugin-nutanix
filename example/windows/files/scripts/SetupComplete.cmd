@echo off
:: Testing for CJ
powershell -NoProfile -ExecutionPolicy Bypass -File "f:\StaticIP.ps1"
:: Enable WinRM
powershell -NoProfile -ExecutionPolicy Bypass -File "f:\EnableWinRMforPacker.ps1"