#!/usr/bin/env python3
"""Stage a mobile build with explicitly configured independent source modules.

No source-specific imports, manifests or repository URLs are written into the
host's tracked module. Configuration is an external JSON array of directories,
with an optional import field (default: the source module path + /mobile).
"""
import argparse
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import tempfile

ANDROID = Path(__file__).resolve().parents[1]
ROOT = ANDROID.parent
GO = os.environ.get("MYGO_PLAYER_GO", "go")


def run(*args, cwd=None):
    return subprocess.run([GO, *args], cwd=cwd, check=True, text=True,
                          stdout=subprocess.PIPE, stderr=subprocess.PIPE).stdout


def prepare(config_path=None):
    configs = []
    config_base = Path.cwd()
    if config_path:
        config_path = Path(config_path).expanduser().resolve(strict=True)
        config_base = config_path.parent
        configs = json.loads(config_path.read_text())
    if not isinstance(configs, list) or len(configs) > 32:
        raise ValueError("Source configuration must be an array of at most 32 entries")
    # Resolve and validate before copying files or modifying a generated module.
    sources = []
    identifiers = set()
    for item in configs:
        directory = Path(item["directory"]).expanduser()
        if not directory.is_absolute():
            directory = config_base / directory
        directory = directory.resolve(strict=True)
        manifest = json.loads((directory / "plugin.json").read_text())
        if not re.fullmatch(r"[a-z0-9][a-z0-9-]{0,63}", manifest.get("id", "")):
            raise ValueError("Invalid source ID")
        if manifest["id"] in identifiers:
            raise ValueError("Duplicate source ID")
        identifiers.add(manifest["id"])
        module = json.loads(run("-C", str(directory), "mod", "edit", "-json"))["Module"]["Path"]
        package = item.get("import", module + "/mobile")
        if not package.startswith(module + "/") or not re.fullmatch(r"[a-zA-Z0-9_./-]+", package):
            raise ValueError("Mobile import must be a package of its source module")
        if not (directory / package[len(module) + 1:]).is_dir():
            raise ValueError("The configured source has no mobile adapter package")
        sources.append((directory, module, package, manifest))
    (ROOT / "build").mkdir(exist_ok=True)
    stage = Path(tempfile.mkdtemp(prefix="android-project-", dir=ROOT / "build"))
    shutil.copytree(ANDROID, stage, dirs_exist_ok=True,
                    ignore=shutil.ignore_patterns(".mygo", "dist", "build", "__pycache__", ".DS_Store"))
    run("mod", "edit", "-replace=mygo-player=" + str(ROOT), cwd=stage)
    config = json.loads((stage / "mygo.json").read_text())
    config["icon"] = str(ROOT / "packaging" / "app-icon.png")
    if os.environ.get("MYGO_ANDROID_KEYSTORE"):
        config["android"]["keystore"] = str(Path(os.environ["MYGO_ANDROID_KEYSTORE"]).expanduser().resolve(strict=True))
        config["android"]["keyAlias"] = os.environ.get("MYGO_ANDROID_KEY_ALIAS", "mygo-player")
    (stage / "mygo.json").write_text(json.dumps(config, ensure_ascii=False, indent=2) + "\n")
    imports = ['"context"', '"encoding/json"', '"mygo-player/android/sourceapi"']
    registrations = []
    licenses = stage / "frontend" / "licenses"
    licenses.mkdir(parents=True, exist_ok=True)
    shutil.copy2(ROOT / "LICENSE", licenses / "MyGo-Player-LICENSE.txt")
    for index, (directory, module, package, manifest) in enumerate(sources):
        run("mod", "edit", "-require=" + module + "@v0.0.0", "-replace=" + module + "=" + str(directory), cwd=stage)
        imports.append(f'mobile{index} "{package}"')
        raw = json.dumps(json.dumps(manifest, ensure_ascii=False, separators=(",", ":")), ensure_ascii=False)
        registrations.append(f'''sourceapi.Register({raw}, func(ctx context.Context, dir string) (sourceapi.Source, error) {{
            return mobile{index}.NewWithManifest(ctx, dir, json.RawMessage({raw}))
        }})''')
        source_licenses = licenses / manifest["id"]
        source_licenses.mkdir()
        for name in ["LICENSE", "THIRD_PARTY_NOTICES.md"]:
            if (directory / name).is_file():
                shutil.copy2(directory / name, source_licenses / name)
        if (directory / "licenses").is_dir():
            shutil.copytree(directory / "licenses", source_licenses / "dependencies")
    if sources:
        generated = "package main\n\nimport (\n" + "\n".join(imports) + "\n)\n\nfunc init() {\n" + "\n".join(registrations) + "\n}\n"
        (stage / "sources_build_generated.go").write_text(generated)
        run("fmt", "./sources_build_generated.go", cwd=stage)
    try:
        run("mod", "tidy", cwd=stage)
    except subprocess.CalledProcessError as exc:
        raise RuntimeError(exc.stderr) from exc
    framework = Path(run("list", "-m", "-f", "{{.Dir}}", "github.com/egoist/mygo", cwd=stage).strip())
    shutil.copy2(framework / "LICENSE", licenses / "MyGo-framework-LICENSE.txt")
    build_info = {"version": config["version"], "sources": [dict(id=m["id"], name=m["name"], version=m["version"]) for _, _, _, m in sources]}
    (stage / "mobile-build-info.json").write_text(json.dumps(build_info, ensure_ascii=False, indent=2) + "\n")
    return stage


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--sources", default=os.environ.get("MYGO_PLAYER_ANDROID_SOURCES"))
    args = parser.parse_args()
    print(prepare(args.sources))
