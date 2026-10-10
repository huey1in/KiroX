"""Render native pages against isolated synthetic data; fail on QML warnings."""
import argparse
import concurrent.futures
import json
import os
from pathlib import Path
import subprocess
import shutil


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--executable", type=Path, default=Path("out/dev/kirox.exe"))
    parser.add_argument("--output", type=Path, default=Path("out/visual"))
    args = parser.parse_args()
    executable = args.executable.resolve()
    output = args.output.resolve()
    environment = os.environ.copy()
    environment["QT_QPA_PLATFORM"] = "offscreen"
    fonts = Path(".tools/smoke-fonts").resolve()
    if os.name == "nt":
        fonts.mkdir(parents=True, exist_ok=True)
        windows_fonts = Path(os.environ.get("WINDIR", "C:/Windows")) / "Fonts"
        for name in ["segoeui.ttf", "segoeuib.ttf", "msyh.ttc", "msyhbd.ttc", "YuGothR.ttc"]:
            original = windows_fonts / name
            if original.is_file() and not (fonts / name).exists():
                shutil.copyfile(original, fonts / name)
    if fonts.is_dir():
        environment["QT_QPA_FONTDIR"] = str(fonts)
    pages = ["overview", "tasks", "accounts", "services", "proxies", "logs", "settings", "about"]
    scenarios = [(theme, language, page, False, False) for theme in ["light", "dark"] for language in ["zh", "en", "ja"] for page in pages]
    scenarios += [(theme, "zh", "settings", True, False) for theme in ["light", "dark"]]

    scenarios += [(theme, "ja", page, False, True) for theme in ["light", "dark"] for page in pages]

    def render(scenario):
        theme, language, page, accessibility, compact = scenario
        name = f"{theme}-{language}-{page}" + ("-reduced" if accessibility else "") + ("-compact" if compact else "")
        root = output / name / "data-home"
        data = root / "data"
        data.mkdir(parents=True, exist_ok=True)

        def save(path, value):
            path.write_text(json.dumps(value, ensure_ascii=False), encoding="utf-8")

        save(root / "settings.json", {"schemaVersion": 4, "runtime": {"theme": theme, "language": language, "autoCheckUpdates": False,
             "reduceMotion": accessibility, "reduceTransparency": accessibility}})
        save(data / "accounts.json", [{"provider": "outlook", "email": f"demo{i}@example.test", "mode": "graph",
             "registered": i % 3 == 0, "success": i % 6 == 0, "addedAt": "2026-10-10"} for i in range(12)])
        save(data / "moemail.json", [{"name": "Primary mailbox", "url": "https://mail.example.test", "apiKey": "synthetic"}])
        save(data / "cloudmail.json", [{"name": "Cloud mailbox", "url": "https://cloud.example.test", "email": "demo@example.test", "password": "synthetic", "domains": ["example.test"]}])
        save(data / "mailnest.json", {"apiKey": "synthetic", "projectCode": "KiroX"})
        save(data / "proxy_pool.json", {"entries": [{"id": f"proxy-{i}", "name": f"Residential {i + 1}", "url": f"http://user:synthetic@proxy{i}.example.test:8080", "weight": 1, "enabled": True} for i in range(4)]})
        image = output / name / "window.png"
        result = subprocess.run([str(executable), "--data-home", str(root), "--page", page, "--screenshot", str(image)] + (["--window-size", "820x580"] if compact else []),
                                env=environment, capture_output=True, timeout=20)
        warnings = result.stderr.decode("utf-8", errors="replace").strip()
        if result.returncode or warnings or not image.is_file():
            raise RuntimeError(f"{name}: exit={result.returncode}: {warnings}")
        return name

    with concurrent.futures.ThreadPoolExecutor(max_workers=3) as pool:
        for name in pool.map(render, scenarios):
            print(f"PASS {name}", flush=True)
    print(f"Rendered {len(scenarios)} isolated native UI scenarios.")


if __name__ == "__main__":
    main()
