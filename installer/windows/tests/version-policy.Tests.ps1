# Casos de comparación de versión del auto-update. Sin red.
# Ejecutar: powershell -NoProfile -ExecutionPolicy Bypass -File .\version-policy.Tests.ps1

$ErrorActionPreference = 'Stop'
$policyPath = Join-Path $PSScriptRoot '..\version-policy.ps1'
. $policyPath

$script:Failed = 0
$script:Passed = 0

function Assert-Equal {
    param(
        [string]$Name,
        [object]$Actual,
        [object]$Expected
    )
    if ($Actual -ne $Expected) {
        $script:Failed++
        Write-Host "FAIL  $Name  expected=$Expected actual=$Actual" -ForegroundColor Red
    } else {
        $script:Passed++
        Write-Host "PASS  $Name" -ForegroundColor Green
    }
}

# Compare-SycronVersion
Assert-Equal 'igual sin prefijo' (Compare-SycronVersion '1.6.13' '1.6.13') 0
Assert-Equal 'igual con prefijo v' (Compare-SycronVersion 'v1.6.13' '1.6.13') 0
Assert-Equal 'igual ambos con v' (Compare-SycronVersion 'v1.6.12' 'V1.6.12') 0
Assert-Equal 'menor' (Compare-SycronVersion '1.6.11' '1.6.12') -1
Assert-Equal 'mayor' (Compare-SycronVersion '1.6.13' '1.6.12') 1

# Resolve-SycronUpdateAction
Assert-Equal 'same 1.6.13 / v1.6.13' (Resolve-SycronUpdateAction -Installed '1.6.13' -Latest 'v1.6.13').Action 'Same'
Assert-Equal 'same v1.6.12 / 1.6.12' (Resolve-SycronUpdateAction -Installed 'v1.6.12' -Latest '1.6.12').Action 'Same'
Assert-Equal 'upgrade 1.6.12 -> 1.6.13' (Resolve-SycronUpdateAction -Installed '1.6.12' -Latest 'v1.6.13').Action 'Upgrade'
Assert-Equal 'downgrade 1.6.13 -> 1.6.12' (Resolve-SycronUpdateAction -Installed '1.6.13' -Latest 'v1.6.12').Action 'SkipDowngrade'
Assert-Equal 'floor latest 1.6.11' (Resolve-SycronUpdateAction -Installed '1.6.13' -Latest 'v1.6.11').Action 'SkipFloor'
Assert-Equal 'floor latest 1.6.0 sin instalada' (Resolve-SycronUpdateAction -Installed '' -Latest '1.6.0').Action 'SkipFloor'
Assert-Equal 'invalid latest' (Resolve-SycronUpdateAction -Installed '1.6.13' -Latest 'not-a-version').Action 'SkipInvalid'
Assert-Equal 'upgrade sin instalada si latest >= floor' (Resolve-SycronUpdateAction -Installed '' -Latest 'v1.6.13').Action 'Upgrade'

Write-Host ""
Write-Host "Passed=$script:Passed Failed=$script:Failed"
if ($script:Failed -gt 0) {
    exit 1
}
exit 0
