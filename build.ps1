param (
    [switch]$Debug = $false,
    [switch]$Sign = $false,
    [switch]$SkipTests = $false,
    [switch]$Package = $false
)

$ErrorActionPreference = "Stop"
Write-Host ">>> Building gnote..." -ForegroundColor Cyan

# 1. Ensure bin dir
if (!(Test-Path "bin")) {
    New-Item -ItemType Directory -Path "bin" -Force | Out-Null
}

# 1.5 Quality gate: gofmt / go vet / go test（与 AGENTS.md 的 CI 门禁保持一致）
if (-not $SkipTests) {
    Write-Host ">>> gofmt check..." -ForegroundColor Cyan
    $unformatted = gofmt -l .
    if ($unformatted) {
        Write-Host ">>> gofmt found unformatted files:" -ForegroundColor Red
        $unformatted | ForEach-Object { Write-Host "    $_" }
        exit 1
    }

    Write-Host ">>> go vet ./..." -ForegroundColor Cyan
    # go vet 会因两条已知的 Win32 消息回调 lParam 指针还原告警返回非零退出码。
    # 只允许这两条固定诊断，其它任何输出（含编译错误）都必须失败。
    $vetOut = @(go vet ./... 2>&1 | ForEach-Object { "$_" } | Where-Object { $_ -ne "" })
    $unexpected = @($vetOut | Where-Object {
        $_ -notmatch '^window\\window\.go:\d+:\d+: possible misuse of unsafe\.Pointer$'
    })
    $known = @($vetOut | Where-Object { $_ -match 'possible misuse of unsafe\.Pointer$' })
    if ($unexpected.Count -gt 0 -or $known.Count -gt 2) {
        $unexpected | ForEach-Object { Write-Host "    $_" -ForegroundColor Red }
        exit 1
    }

    Write-Host ">>> go test ./..." -ForegroundColor Cyan
    go test ./...
    if ($LASTEXITCODE -ne 0) { exit 1 }
}

# 2. Compile Windows PE resource section (Icon + Manifest + VERSIONINFO)
if (Get-Command windres -ErrorAction SilentlyContinue) {
    Write-Host ">>> Compiling PE resources (Icon + Manifest + VERSIONINFO)..." -ForegroundColor DarkGray
    windres -i cmd/gnote/gnote.rc -O coff -o cmd/gnote/gnote_windows_amd64.syso
}

# 3. Build main executable (-trimpath strips local paths, -H windowsgui hides console, -s -w reduces size)
$ldflags = "-H windowsgui -s -w"
if ($Debug) {
    $ldflags = ""
    Write-Host ">>> [Debug Mode] Console output enabled..." -ForegroundColor Yellow
}

Write-Host ">>> Compiling gnote.exe..." -ForegroundColor Cyan
go build -trimpath -ldflags="$ldflags" -o bin/gnote.exe ./cmd/gnote

# 4. Optional: Authenticode code signing
if ($Sign) {
    Write-Host ">>> Performing Authenticode code signing..." -ForegroundColor Cyan
    $cert = Get-ChildItem Cert:\CurrentUser\My -CodeSigningCert | Where-Object { $_.Subject -like "*gnote*" } | Select-Object -First 1
    if (-not $cert) {
        Write-Host ">>> Generating self-signed Code Signing Certificate (gnote)..." -ForegroundColor Yellow
        $cert = New-SelfSignedCertificate -Type CodeSigningCert -Subject "CN=gnote Code Signing, O=gnote, C=CN" -CertStoreLocation Cert:\CurrentUser\My -NotAfter (Get-Date).AddYears(5)
        Export-Certificate -Cert $cert -FilePath "bin/gnote_CodeSigning.cer" -Force | Out-Null
        Write-Host ">>> Exported public certificate to bin/gnote_CodeSigning.cer" -ForegroundColor DarkGray
    }
    Set-AuthenticodeSignature -FilePath "bin/gnote.exe" -Certificate $cert | Out-Null
    Write-Host ">>> Code signing completed successfully!" -ForegroundColor Green
}

# 5. Copy to root directory
Copy-Item -Force bin/gnote.exe gnote.exe

# 6. Unblock Mark of the Web
Unblock-File -Path "bin/gnote.exe" -ErrorAction SilentlyContinue
Unblock-File -Path "gnote.exe" -ErrorAction SilentlyContinue

Write-Host ">>> Build successful: bin/gnote.exe & gnote.exe (Native Windows GUI, terminal hidden, manifest & version info embedded)" -ForegroundColor Green

# 7. Optional: Package portable release ZIP
if ($Package) {
    Write-Host ">>> Packaging release zip..." -ForegroundColor Cyan
    $pkgDir = "bin/gnote-portable"
    if (Test-Path $pkgDir) {
        Remove-Item -Recurse -Force $pkgDir
    }
    New-Item -ItemType Directory -Path $pkgDir -Force | Out-Null
    Copy-Item "bin/gnote.exe" "$pkgDir/gnote.exe"
    Copy-Item "cmd/gnote/font-LICENSE.txt" "$pkgDir/font-LICENSE.txt"

    if (Test-Path "bin/gnote_CodeSigning.cer") {
        Copy-Item "bin/gnote_CodeSigning.cer" "$pkgDir/gnote_CodeSigning.cer"
    }

    # Generate unblock launcher script
    $batLines = @(
        "@echo off",
        "chcp 65001 >nul",
        "powershell -NoProfile -ExecutionPolicy Bypass -Command `"Unblock-File -Path '%~dp0gnote.exe'`"",
        "start `"`" `"%~dp0gnote.exe`""
    )
    [System.IO.File]::WriteAllLines("$pkgDir/run_unblocked.bat", $batLines, [System.Text.Encoding]::UTF8)

    # Generate README
    $readmeText = @"
======================================================================
  gnote - Fast Notepad and Code Editor
  Author: jacksalad
  https://github.com/jacksalad/gnote
======================================================================

If Windows Defender SmartScreen warns about an unrecognized app:
1. Click 'More info' -> Click 'Run anyway', OR
2. Right click gnote.exe -> Properties -> Check 'Unblock' -> OK, OR
3. Double click run_unblocked.bat to launch directly.

======================================================================
"@
    [System.IO.File]::WriteAllText("$pkgDir/README.txt", $readmeText, [System.Text.Encoding]::UTF8)

    $zipPath = "bin/gnote-v1.0.0-windows-amd64.zip"
    if (Test-Path $zipPath) {
        Remove-Item -Force $zipPath
    }
    Compress-Archive -Path "$pkgDir/*" -DestinationPath $zipPath -Force
    Remove-Item -Recurse -Force $pkgDir
    Write-Host ">>> Release package created: $zipPath" -ForegroundColor Green
}
