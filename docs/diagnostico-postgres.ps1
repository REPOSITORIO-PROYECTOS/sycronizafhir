$ErrorActionPreference = "Stop"

function Write-Step {
    param([string]$Message)
    Write-Host "`n==> $Message" -ForegroundColor Cyan
}

function Write-Ok {
    param([string]$Message)
    Write-Host "[OK] $Message" -ForegroundColor Green
}

function Write-WarnMsg {
    param([string]$Message)
    Write-Host "[WARN] $Message" -ForegroundColor Yellow
}

function Write-ErrMsg {
    param([string]$Message)
    Write-Host "[ERROR] $Message" -ForegroundColor Red
}

function Load-DotEnv {
    param([string]$Path)
    if (!(Test-Path $Path)) {
        throw "No se encontro archivo .env en: $Path"
    }

    Get-Content $Path | ForEach-Object {
        $line = $_.Trim()
        if ($line -eq "" -or $line.StartsWith("#")) { return }
        $parts = $line -split "=", 2
        if ($parts.Length -ne 2) { return }
        $key = $parts[0].Trim()
        $value = $parts[1].Trim()
        [Environment]::SetEnvironmentVariable($key, $value, "Process")
    }
}

function Parse-ConnectionString {
    param([string]$ConnectionString)
    $uri = [System.Uri]$ConnectionString

    $userInfo = $uri.UserInfo -split ":", 2
    $username = $userInfo[0]
    $password = if ($userInfo.Length -gt 1) { $userInfo[1] } else { "" }

    $dbName = $uri.AbsolutePath.TrimStart("/")
    $query = [System.Web.HttpUtility]::ParseQueryString($uri.Query)
    $sslMode = $query.Get("sslmode")
    if ([string]::IsNullOrWhiteSpace($sslMode)) { $sslMode = "disable" }

    return @{
        Host     = $uri.Host
        Port     = if ($uri.Port -gt 0) { $uri.Port } else { 5432 }
        User     = $username
        Password = $password
        Database = $dbName
        SSLMode  = $sslMode
    }
}

Write-Step "Cargando .env"
$envPath = Join-Path $PSScriptRoot ".env"
Load-DotEnv -Path $envPath
Write-Ok ".env cargado: $envPath"

$localUrl = $env:LOCAL_POSTGRES_URL
if ([string]::IsNullOrWhiteSpace($localUrl)) {
    throw "LOCAL_POSTGRES_URL no esta definido en .env"
}

Write-Step "Parseando LOCAL_POSTGRES_URL"
$parsed = Parse-ConnectionString -ConnectionString $localUrl
Write-Ok "Host: $($parsed.Host), Port: $($parsed.Port), DB: $($parsed.Database), User: $($parsed.User), SSL: $($parsed.SSLMode)"

Write-Step "Probando conectividad TCP"
$tcpResult = Test-NetConnection -ComputerName $parsed.Host -Port $parsed.Port -WarningAction SilentlyContinue
if ($tcpResult.TcpTestSucceeded) {
    Write-Ok "Puerto accesible ($($parsed.Host):$($parsed.Port))"
} else {
    Write-ErrMsg "No hay conectividad TCP a $($parsed.Host):$($parsed.Port)"
    Write-WarnMsg "Si usas localhost, proba cambiar a 127.0.0.1 en LOCAL_POSTGRES_URL"
}

Write-Step "Probando autenticacion con psql"
$psqlCmd = Get-Command psql -ErrorAction SilentlyContinue
if (-not $psqlCmd) {
    Write-WarnMsg "psql no esta en PATH. Instala PostgreSQL client o agrega psql al PATH."
    Write-WarnMsg "No se puede validar autenticacion SQL sin psql."
    exit 2
}

$env:PGPASSWORD = $parsed.Password
$psqlArgs = @(
    "-h", $parsed.Host,
    "-p", "$($parsed.Port)",
    "-U", $parsed.User,
    "-d", $parsed.Database,
    "-c", "SELECT current_user, current_database(), version();"
)

& psql @psqlArgs
$exitCode = $LASTEXITCODE
Remove-Item Env:\PGPASSWORD -ErrorAction SilentlyContinue

if ($exitCode -eq 0) {
    Write-Ok "Autenticacion PostgreSQL OK"
    exit 0
}

Write-ErrMsg "Autenticacion PostgreSQL fallo (exit code $exitCode)"
Write-WarnMsg "Revisa usuario/clave en LOCAL_POSTGRES_URL y pg_hba.conf"
exit $exitCode

