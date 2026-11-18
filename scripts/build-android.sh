#!/bin/sh
set -eu
ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
ENV_FILE=${MYGO_PLAYER_ANDROID_ENV:-"$ROOT/../.tools/android-tools-setup.env"}
if [ -f "$ENV_FILE" ]; then
    . "$ENV_FILE"
fi
MYGO_CLI=${MYGO_PLAYER_MYGO:-mygo}
command -v "$MYGO_CLI" >/dev/null 2>&1 || { echo 'Install MyGo v0.10.0 CLI or set MYGO_PLAYER_MYGO.' >&2; exit 1; }
command -v go >/dev/null 2>&1 || { echo 'Go is required.' >&2; exit 1; }
command -v apksigner >/dev/null 2>&1 || { echo 'Android build-tools apksigner is required.' >&2; exit 1; }
command -v aapt2 >/dev/null 2>&1 || { echo 'Android build-tools aapt2 is required.' >&2; exit 1; }
OUTPUT_DIR=${MYGO_PLAYER_OUTPUT_DIR:-"${HOME:?}/Desktop/MyGo Player 打包"}
PLATFORM=${MYGO_PLAYER_ANDROID_PLATFORM:-android/arm64}
case "$PLATFORM" in
    android/arm64) ARCH=arm64 ;;
    android/arm) ARCH=arm ;;
    android/amd64) ARCH=amd64 ;;
    android) ARCH=universal ;;
    *) echo 'Unsupported Android platform.' >&2; exit 1 ;;
esac
SOURCES=${MYGO_PLAYER_ANDROID_SOURCES:-}
if [ -z "$SOURCES" ] && [ -f "$ROOT/../.android-sources.json" ]; then
    SOURCES="$ROOT/../.android-sources.json"
fi
if [ -n "$SOURCES" ]; then
    printf 'Android sources: %s\n' "$SOURCES"
    PROJECT=$(python3 "$ROOT/android/scripts/prepare-project.py" --sources "$SOURCES")
else
    printf 'Android sources: none (host shell only)\n'
    PROJECT=$(python3 "$ROOT/android/scripts/prepare-project.py")
fi
printf 'Android build project: %s\n' "$PROJECT"
"$MYGO_CLI" build -platform "$PLATFORM" "$PROJECT"
VERSION=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["version"])' "$PROJECT/mygo.json")
APK="$PROJECT/dist/android/mygo-player-$VERSION.apk"
AAB="$PROJECT/dist/android/mygo-player-$VERSION.aab"
apksigner verify --verbose "$APK"
python3 - "$APK" "$VERSION" "$ARCH" <<'PY'
import re, subprocess, sys, zipfile
apk, version, arch = sys.argv[1:]
with zipfile.ZipFile(apk) as package:
    if package.testzip():
        raise SystemExit('APK ZIP integrity check failed')
    names = package.namelist()
    abis = {'arm64':'arm64-v8a','arm':'armeabi-v7a','amd64':'x86_64'}
    needed = list(abis.values()) if arch == 'universal' else [abis[arch]]
    for abi in needed:
        if not any(n.startswith('lib/' + abi + '/') and n.endswith('.so') for n in names):
            raise SystemExit('Missing native library: ' + abi)
badging = subprocess.check_output(['aapt2','dump','badging',apk], text=True)
if f"versionName='{version}'" not in badging or "minSdkVersion:'26'" not in badging:
    raise SystemExit('Unexpected Android version/minSdk')
manifest = subprocess.check_output(['aapt2','dump','xmltree','--file','AndroidManifest.xml',apk], text=True)
if 'networkSecurityConfig' not in manifest or 'android.permission.INTERNET' not in manifest:
    raise SystemExit('Missing local-media network configuration')
print('APK signature, ZIP integrity, ABI, version, minSdk and network configuration verified')
PY
mkdir -p "$OUTPUT_DIR"
BASE="mygo-player-$VERSION-android-$ARCH"
cp "$APK" "$OUTPUT_DIR/$BASE.apk"
cp "$AAB" "$OUTPUT_DIR/$BASE.aab"
cp "$PROJECT/mobile-build-info.json" "$OUTPUT_DIR/$BASE-build-info.json"
python3 - "$OUTPUT_DIR" "$BASE" <<'PY'
import hashlib
from pathlib import Path
import sys
root, base = Path(sys.argv[1]), sys.argv[2]
lines=[]
for suffix in ['.apk','.aab']:
    p=root/(base+suffix)
    lines.append(hashlib.sha256(p.read_bytes()).hexdigest()+'  '+p.name)
(root/(base+'-SHA256SUMS.txt')).write_text('\n'.join(lines)+'\n')
print(root/(base+'.apk'))
PY
