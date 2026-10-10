"""Start the installed app with development SDK locations removed."""
import json
import os
from contextlib import contextmanager
from pathlib import Path
import subprocess
import sys
import tempfile


@contextmanager
def hide_development_sdks():
    """Also catch absolute RPATH/install-name references back into the SDK."""
    workspace = Path(__file__).resolve().parents[1]
    moved = []
    try:
        for relative in [".tools/qt", ".tools/curl", "out/release/_deps"]:
            source = (workspace / relative).resolve()
            destination = source.with_name(source.name + ".kirox-package-verification")
            if not source.is_relative_to(workspace) or not destination.is_relative_to(workspace):
                raise RuntimeError("SDK verification path is outside the workspace")
            if source.exists():
                if destination.exists():
                    raise RuntimeError(f"An unfinished package check exists: {destination}")
                source.rename(destination)
                moved.append((source, destination))
        yield
    finally:
        for source, destination in reversed(moved):
            destination.rename(source)


def main():
    package = Path(sys.argv[1]).resolve()
    if sys.platform == "win32":
        executable = package / "bin/kirox.exe"
    elif sys.platform == "darwin":
        executable = package / "KiroX.app/Contents/MacOS/KiroX"
    else:
        executable = package / "bin/kirox"
    env = os.environ.copy()
    env["PATH"] = os.pathsep.join(part for part in env.get("PATH", "").split(os.pathsep) if ".tools" not in part)
    for key in ["QT_PLUGIN_PATH", "QT_QPA_PLATFORM_PLUGIN_PATH", "QML_IMPORT_PATH", "QML2_IMPORT_PATH", "DYLD_LIBRARY_PATH", "DYLD_FRAMEWORK_PATH", "LD_LIBRARY_PATH"]:
        env.pop(key, None)
    env["QT_QPA_PLATFORM"] = "offscreen"
    env["QT_QUICK_BACKEND"] = "software"
    with tempfile.TemporaryDirectory() as temporary:
        root = Path(temporary)
        (root / "settings.json").write_text(json.dumps({"runtime": {"language": "en", "autoCheckUpdates": False}}), encoding="utf-8")
        image = root / "window.png"
        with hide_development_sdks():
            completed = subprocess.run([str(executable), "--data-home", str(root), "--screenshot", str(image)], env=env, capture_output=True, timeout=20)
        if completed.returncode or not image.is_file():
            raise RuntimeError(completed.stderr.decode(errors="replace") or f"Application exit: {completed.returncode}")
        print("Installed native application rendered successfully with development SDKs hidden and search paths removed.")


if __name__ == "__main__":
    main()
