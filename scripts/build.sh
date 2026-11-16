#!/bin/sh
set -eu
ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd -P)
cd "$ROOT"
GO_BIN=${GO_BIN:-go}
if ! command -v "$GO_BIN" >/dev/null 2>&1; then
  if [ -x "$ROOT/../.tools/go/bin/go" ]; then GO_BIN="$ROOT/../.tools/go/bin/go"; else printf '%s\n' 'Install Go 1.27.3+ or set GO_BIN.' >&2; exit 1; fi
fi
mkdir -p build
CGO_ENABLED=0 "$GO_BIN" build -trimpath -tags mygo_noinspector -ldflags '-s -w' -o build/mygo-player .
if [ "$(uname -s)" = Darwin ]; then
  APP="$ROOT/build/MyGo Player.app"
  # Replace the known bundle completely, so old bundled plugins cannot survive
  # an upgrade of the source tree. Refuse a redirected build or bundle path.
  BUILD_RESOLVED=$(CDPATH= cd -- "$ROOT/build" && pwd -P)
  [ "$BUILD_RESOLVED" = "$ROOT/build" ] || { printf '%s\n' 'Refusing redirected build directory.' >&2; exit 1; }
  if [ -e "$APP" ] || [ -L "$APP" ]; then
    APP_RESOLVED=$(CDPATH= cd -- "$APP" && pwd -P)
    [ "$APP_RESOLVED" = "$ROOT/build/MyGo Player.app" ] || { printf '%s\n' 'Refusing redirected application bundle.' >&2; exit 1; }
    rm -rf "$APP"
  fi
  mkdir -p "$APP/Contents/MacOS" "$APP/Contents/Resources"
  cp build/mygo-player "$APP/Contents/MacOS/mygo-player"
  cp packaging/Info.plist "$APP/Contents/Info.plist"
  cp packaging/AppIcon.icns "$APP/Contents/Resources/AppIcon.icns"
  if [ -n "${MYGO_PLAYER_BUNDLE_FFMPEG:-}" ]; then
    cp "$MYGO_PLAYER_BUNDLE_FFMPEG" "$APP/Contents/MacOS/ffmpeg"
    chmod 755 "$APP/Contents/MacOS/ffmpeg"
  fi
  cp LICENSE THIRD_PARTY_NOTICES.md "$APP/Contents/Resources/"
  cp -R licenses "$APP/Contents/Resources/"
  if command -v codesign >/dev/null 2>&1; then codesign --force --deep --sign - "$APP"; fi
  printf 'Built: %s\n' "$APP"
else
  printf 'Built: %s/build/mygo-player\n' "$ROOT"
fi
