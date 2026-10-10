param([ValidateSet("dev", "release")][string]$Preset = "dev", [switch]$Deploy, [ValidateRange(1, 64)][int]$Jobs = 6)
$ErrorActionPreference = "Stop"
$repoRoot = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot ".."))
Set-Location -LiteralPath $repoRoot
$qtRoot = Join-Path $repoRoot ".tools/qt/6.8.3/mingw_64"
$compilerBin = Join-Path $repoRoot ".tools/qt/Tools/mingw1310_64/bin"
$toolBin = Join-Path $repoRoot ".tools/python/Scripts"
$env:PATH = "$qtRoot/bin;$compilerBin;$toolBin;$env:PATH"
$cmakePath = Join-Path $toolBin "cmake.exe"
if (-not (Test-Path -LiteralPath $cmakePath)) { throw "Run scripts/bootstrap-cpp.ps1 first" }
& $cmakePath --preset $Preset "-DCMAKE_PREFIX_PATH=$qtRoot" "-DCMAKE_CXX_COMPILER=$compilerBin/g++.exe"
if ($LASTEXITCODE -ne 0) { throw "CMake configuration failed" }
& $cmakePath --build --preset $Preset --parallel $Jobs
if ($LASTEXITCODE -ne 0) { throw "C++ build failed" }
& (Join-Path $toolBin "ctest.exe") --preset $Preset
if ($LASTEXITCODE -ne 0) { throw "Tests failed" }
if ($Deploy) {
    & $cmakePath --install (Join-Path $repoRoot "out/$Preset") --prefix (Join-Path $repoRoot "out/package")
    if ($LASTEXITCODE -ne 0) { throw "Deployment failed" }
}
