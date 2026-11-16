#!/bin/sh
set -eu
ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd -P)
cd "$ROOT"
GO_BIN=${GO_BIN:-go}
if ! command -v "$GO_BIN" >/dev/null 2>&1; then
  if [ -x "$ROOT/../.tools/go/bin/go" ]; then GO_BIN="$ROOT/../.tools/go/bin/go"; else printf '%s\n' 'Install Go 1.27.3+ or set GO_BIN.' >&2; exit 1; fi
fi
: "${MYGO_PLAYER_BUNDLE_FFMPEG:?Set MYGO_PLAYER_BUNDLE_FFMPEG to a Windows x64 ffmpeg.exe}"
: "${WEBVIEW2_BOOTSTRAPPER:?Set WEBVIEW2_BOOTSTRAPPER to the official MicrosoftEdgeWebview2Setup.exe}"
[ -f "$WEBVIEW2_BOOTSTRAPPER" ] || { printf '%s\n' 'WebView2 bootstrapper not found.' >&2; exit 1; }
VERSION=$(python3 -c 'import plistlib; print(plistlib.load(open("packaging/Info.plist", "rb"))["CFBundleShortVersionString"])')
mkdir -p build
STAGE=$(mktemp -d "$ROOT/build/windows-package.XXXXXX")
export GOOS=windows GOARCH=amd64 CGO_ENABLED=0
"$GO_BIN" build -trimpath -tags mygo_noinspector -ldflags '-s -w -H=windowsgui' -o "$STAGE/mygo-player.exe" .
cp "$WEBVIEW2_BOOTSTRAPPER" "$STAGE/MicrosoftEdgeWebview2Setup.exe"
cp "$MYGO_PLAYER_BUNDLE_FFMPEG" "$STAGE/ffmpeg.exe"
cp LICENSE THIRD_PARTY_NOTICES.md "$STAGE/"
cp -R licenses "$STAGE/"
cp packaging/windows-README.txt "$STAGE/README.txt"
python3 - "$STAGE" "$ROOT/build/mygo-player-$VERSION-windows-amd64.zip" <<'PY'
from pathlib import Path
import struct, sys, zipfile
stage, output = map(Path, sys.argv[1:])
for name, machine in [('mygo-player.exe', 0x8664), ('ffmpeg.exe', 0x8664), ('MicrosoftEdgeWebview2Setup.exe', None)]:
    p = stage / name
    data = p.read_bytes()
    assert data[:2] == b'MZ', name
    offset = struct.unpack_from('<I', data, 0x3c)[0]
    assert data[offset:offset+4] == b'PE\0\0', name
    if machine:
        assert struct.unpack_from('<H', data, offset+4)[0] == machine, name
with zipfile.ZipFile(output, 'w', zipfile.ZIP_DEFLATED, compresslevel=9) as archive:
    for p in sorted(stage.rglob('*')):
        if p.is_file():
            archive.write(p, (Path('MyGo Player') / p.relative_to(stage)).as_posix())
print('Built:', output)
PY
if command -v makensis >/dev/null 2>&1; then
  makensis -V2 "-DAPP_VERSION=$VERSION" "-DAPP_DIR=$STAGE" "-DOUTPUT_FILE=$ROOT/build/mygo-player-$VERSION-windows-amd64-setup.exe" packaging/windows.nsi
else
  printf '%s\n' 'NSIS not found; portable ZIP built without a setup executable.'
fi
printf 'Staged files: %s\n' "$STAGE"
