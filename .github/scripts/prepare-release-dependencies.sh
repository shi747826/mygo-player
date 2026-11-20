#!/usr/bin/env bash
set -euo pipefail

platform=$1
output_dir=$2
mkdir -p "$output_dir"

# Keep wheel versions and digests in sync with THIRD_PARTY_NOTICES.md.
case "$platform" in
  darwin-arm64)
    wheel_url=https://files.pythonhosted.org/packages/2b/47/9f9b654f853598bae0850fa22bba168103e45d31b5a2a893ed474f28d54a/imageio_ffmpeg-0.8.1-py3-none-macosx_12_0_arm64.whl
    wheel_sha256=5636d691e36e5274b13fe38182b1f627b2e35dcecc63944012c2470f9484fc45
    binary=ffmpeg-macos-aarch64-v9.0
    ffmpeg=ffmpeg
    ;;
  windows-amd64)
    wheel_url=https://files.pythonhosted.org/packages/82/0c/a1d49f3dd2688bd750b4986f8e271ad11f8e729a896070ea25f587b44da8/imageio_ffmpeg-0.8.1-py3-none-win_amd64.whl
    wheel_sha256=f27ca4cb85e2d4946deedbce1a81245dfd942a2bcf94b53676fb3956aa95bf1b
    binary=ffmpeg-win-x86_64-v9.0.2.exe
    ffmpeg=ffmpeg.exe
    ;;
  *)
    printf 'Unsupported release platform: %s\n' "$platform" >&2
    exit 1
    ;;
esac

curl --fail --silent --show-error --location --retry 3 --proto '=https' \
  --output "$output_dir/imageio-ffmpeg.whl" "$wheel_url"
printf '%s  %s\n' "$wheel_sha256" "$output_dir/imageio-ffmpeg.whl" | shasum -a 256 -c -
unzip -p "$output_dir/imageio-ffmpeg.whl" "imageio_ffmpeg/binaries/$binary" > "$output_dir/$ffmpeg"
chmod 755 "$output_dir/$ffmpeg"

if [[ "$platform" == windows-amd64 ]]; then
  # Microsoft services this Evergreen bootstrapper behind a stable URL.
  curl --fail --silent --show-error --location --retry 3 --proto '=https' \
    --output "$output_dir/MicrosoftEdgeWebview2Setup.exe" 'https://go.microsoft.com/fwlink/p/?LinkId=2124703'
fi
