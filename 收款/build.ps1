param(
    [string]$CredentialConfig = (Join-Path $PSScriptRoot '.credentials/config.json'),
    [string]$GoExecutable = '',
    [ValidateSet('windows', 'linux')][string]$TargetOS = 'windows',
    [ValidateSet('amd64', 'arm64')][string]$TargetArch = 'amd64'
)
$ErrorActionPreference = 'Stop'

if (-not $GoExecutable) {
    $goCommand = Get-Command go -ErrorAction SilentlyContinue
    if ($goCommand) { $GoExecutable = $goCommand.Source }
    elseif (Test-Path -LiteralPath 'D:\Program Files\Go\bin\go.exe') { $GoExecutable = 'D:\Program Files\Go\bin\go.exe' }
    else { throw '找不到 Go，请安装 Go 1.25+ 或通过 -GoExecutable 指定。' }
}

$credentialPath = (Resolve-Path -LiteralPath $CredentialConfig).Path
$credentialDirectory = Split-Path -Parent $credentialPath
$settings = [IO.File]::ReadAllText($credentialPath) | ConvertFrom-Json
if ([string]$settings.app_id -notmatch '^\d+$') { throw '凭据配置中的 app_id 无效。' }

function Read-KeyFile([string]$FileName) {
    $keyPath = if ([IO.Path]::IsPathRooted($FileName)) { $FileName } else { Join-Path $credentialDirectory $FileName }
    return [IO.File]::ReadAllText($keyPath).Trim()
}

$embedded = [ordered]@{
    app_id = [string]$settings.app_id
    private_key = Read-KeyFile ([string]$settings.app_private_key_file)
    public_key = Read-KeyFile ([string]$settings.alipay_public_key_file)
}
$utf8 = [Text.UTF8Encoding]::new($false)
$assetDirectory = Join-Path $PSScriptRoot 'internal/credentials/assets'
[IO.Directory]::CreateDirectory($assetDirectory) | Out-Null
[IO.File]::WriteAllText((Join-Path $assetDirectory 'credentials.local.json'), ($embedded | ConvertTo-Json), $utf8)

$runtimeConfig = Join-Path $PSScriptRoot 'config.yaml'
if (-not (Test-Path -LiteralPath $runtimeConfig)) {
    $secretBytes = New-Object byte[] 32
    $rng = [Security.Cryptography.RandomNumberGenerator]::Create()
    try { $rng.GetBytes($secretBytes) } finally { $rng.Dispose() }
    $secret = [BitConverter]::ToString($secretBytes).Replace('-', '').ToLowerInvariant()
    $template = [IO.File]::ReadAllText((Join-Path $PSScriptRoot 'config.example.yaml'))
    [IO.File]::WriteAllText($runtimeConfig, $template.Replace('__GENERATE_SECRET__', $secret), $utf8)
    Write-Host '已生成 config.yaml（随机共享密钥只保存在文件中）。'
}

$outputDirectory = Join-Path $PSScriptRoot '.build'
[IO.Directory]::CreateDirectory($outputDirectory) | Out-Null
$fileName = if ($TargetOS -eq 'windows') { "receipt-service-$TargetArch.exe" } else { "receipt-service-linux-$TargetArch" }
$outputFile = Join-Path $outputDirectory $fileName
$oldModule = $env:GO111MODULE
$oldOS = $env:GOOS
$oldArch = $env:GOARCH
$oldCGO = $env:CGO_ENABLED
Push-Location $PSScriptRoot
try {
    $env:GO111MODULE = 'on'
    $env:GOOS = $TargetOS
    $env:GOARCH = $TargetArch
    $env:CGO_ENABLED = '0'
    & $GoExecutable build -trimpath -tags nomsgpack -ldflags '-s -w' -o $outputFile .
    if ($LASTEXITCODE -ne 0) { throw 'Go 编译失败。' }
    Write-Host "构建完成：$outputFile"
} finally {
    Pop-Location
    $env:GO111MODULE = $oldModule
    $env:GOOS = $oldOS
    $env:GOARCH = $oldArch
    $env:CGO_ENABLED = $oldCGO
}
