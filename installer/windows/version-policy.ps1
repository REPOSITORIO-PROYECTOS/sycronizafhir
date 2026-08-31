# Política de auto-update sycronizafhir.
# Floor 1.6.12: debajo reaparece INSERT en pedidos ERP (worker pedidos_tienda).
# Compara semver; ignora prefijo v/V. Sin I/O ni red.

$script:SycronUpdateFloor = '1.6.12'

function Normalize-SycronVersion {
    param([Parameter(Mandatory = $false)][string]$Version)

    if ([string]::IsNullOrWhiteSpace($Version)) {
        return ''
    }
    $trimmed = $Version.Trim()
    if ($trimmed -match '^[vV](.+)$') {
        $trimmed = $Matches[1].Trim()
    }
    if ($trimmed -match '^([0-9]+(?:\.[0-9]+){0,2})') {
        return $Matches[1]
    }
    return ''
}

function Compare-SycronVersion {
    param(
        [Parameter(Mandatory = $true)][AllowEmptyString()][string]$Left,
        [Parameter(Mandatory = $true)][AllowEmptyString()][string]$Right
    )

    $normLeft = Normalize-SycronVersion $Left
    $normRight = Normalize-SycronVersion $Right
    if ($normLeft -eq '' -and $normRight -eq '') {
        return 0
    }
    if ($normLeft -eq '') {
        return -1
    }
    if ($normRight -eq '') {
        return 1
    }

    $partsLeft = @($normLeft -split '\.')
    $partsRight = @($normRight -split '\.')
    $n = [Math]::Max($partsLeft.Count, $partsRight.Count)
    for ($i = 0; $i -lt $n; $i++) {
        $a = 0
        $b = 0
        if ($i -lt $partsLeft.Count) {
            [void][int]::TryParse($partsLeft[$i], [ref]$a)
        }
        if ($i -lt $partsRight.Count) {
            [void][int]::TryParse($partsRight[$i], [ref]$b)
        }
        if ($a -lt $b) { return -1 }
        if ($a -gt $b) { return 1 }
    }
    return 0
}

function Resolve-SycronUpdateAction {
    param(
        [Parameter(Mandatory = $false)][string]$Installed = '',
        [Parameter(Mandatory = $true)][AllowEmptyString()][string]$Latest,
        [Parameter(Mandatory = $false)][string]$Floor = $script:SycronUpdateFloor
    )

    $normLatest = Normalize-SycronVersion $Latest
    $normInstalled = Normalize-SycronVersion $Installed
    $normFloor = Normalize-SycronVersion $Floor

    if ($normLatest -eq '') {
        return [pscustomobject]@{
            Action           = 'SkipInvalid'
            Message          = "SKIP invalid latest='$Latest'"
            InstalledNormalized = $normInstalled
            LatestNormalized = $normLatest
        }
    }

    if ((Compare-SycronVersion $normLatest $normFloor) -lt 0) {
        return [pscustomobject]@{
            Action           = 'SkipFloor'
            Message          = "SKIP floor: latest $Latest < $Floor"
            InstalledNormalized = $normInstalled
            LatestNormalized = $normLatest
        }
    }

    if ($normInstalled -ne '' -and ((Compare-SycronVersion $normLatest $normInstalled) -lt 0)) {
        return [pscustomobject]@{
            Action           = 'SkipDowngrade'
            Message          = "SKIP downgrade: instalada $Installed > latest $Latest"
            InstalledNormalized = $normInstalled
            LatestNormalized = $normLatest
        }
    }

    if ($normInstalled -ne '' -and ((Compare-SycronVersion $normLatest $normInstalled) -eq 0)) {
        return [pscustomobject]@{
            Action           = 'Same'
            Message          = "same version $normInstalled"
            InstalledNormalized = $normInstalled
            LatestNormalized = $normLatest
        }
    }

    return [pscustomobject]@{
        Action           = 'Upgrade'
        Message          = "upgrade $Installed -> $Latest"
        InstalledNormalized = $normInstalled
        LatestNormalized = $normLatest
    }
}
