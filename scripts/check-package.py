"""Start the installed app with development SDK locations removed."""
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile


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
    for key in ["QT_PLUGIN_PATH", "QML_IMPORT_PATH", "QML2_IMPORT_PATH", "DYLD_LIBRARY_PATH", "LD_LIBRARY_PATH"]:
        env.pop(key, None)
    env["QT_QPA_PLATFORM"] = "offscreen"
    env["QT_QUICK_BACKEND"] = "software"
    with tempfile.TemporaryDirectory() as temporary:
        root = Path(temporary)
        (root / "settings.json").write_text(json.dumps({"runtime": {"language": "en", "autoCheckUpdates": False}}), encoding="utf-8")
        image = root / "window.png"
        completed = subprocess.run([str(executable), "--data-home", str(root), "--screenshot", str(image)], env=env, capture_output=True, timeout=20)
        if completed.returncode or not image.is_file():
            raise RuntimeError(completed.stderr.decode(errors="replace") or f"Application exit: {completed.returncode}")
        print("Installed native application rendered successfully without SDK search paths.")


if __name__ == "__main__":
    main()
