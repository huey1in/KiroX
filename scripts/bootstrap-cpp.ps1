param([string]$QtVersion = "6.8.3")
$ErrorActionPreference = "Stop"
$repoRoot = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot ".."))
Set-Location -LiteralPath $repoRoot
$pythonPath = Join-Path $repoRoot ".tools/python/Scripts/python.exe"
if (-not (Test-Path -LiteralPath $pythonPath)) {
    python -m venv (Join-Path $repoRoot ".tools/python")
    if ($LASTEXITCODE -ne 0) { throw "Python environment creation failed" }
}
& $pythonPath -m pip install aqtinstall==3.3.0 cmake==3.31.6 ninja==1.11.1.3
if ($LASTEXITCODE -ne 0) { throw "Build tool installation failed" }
$qtRoot = Join-Path $repoRoot ".tools/qt"
$qtBin = Join-Path $qtRoot "$QtVersion/mingw_64/bin"
if (-not (Test-Path -LiteralPath (Join-Path $qtBin "moc.exe"))) {
    & $pythonPath -m aqt install-qt windows desktop $QtVersion win64_mingw -O $qtRoot -m qtshadertools qtmultimedia --archives qtbase qtdeclarative qtsvg qttools --timeout 20
    if ($LASTEXITCODE -ne 0) { throw "Qt installation failed" }
}
if (-not (Test-Path -LiteralPath (Join-Path $qtBin "Qt6Multimedia.dll"))) {
    & $pythonPath -m aqt install-qt windows desktop $QtVersion win64_mingw -O $qtRoot -m qtmultimedia --noarchives --timeout 20
    if ($LASTEXITCODE -ne 0) { throw "Qt multimedia installation failed" }
}
$compilerPath = Join-Path $qtRoot "Tools/mingw1310_64/bin/g++.exe"
if (-not (Test-Path -LiteralPath $compilerPath)) {
    & $pythonPath -m aqt install-tool windows desktop tools_mingw1310 qt.tools.win64_mingw1310 -O $qtRoot --timeout 20
    if ($LASTEXITCODE -ne 0) { throw "MinGW installation failed" }
}
Write-Host "Build tools are ready. Run scripts/build-cpp.ps1."
