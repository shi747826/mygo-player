#!/usr/bin/env python3
"""Download the pinned HLS.js package and verify its npm SHA-512 integrity."""
import base64
import hashlib
import io
from pathlib import Path
import tarfile
import urllib.request

VERSION = "1.7.4"
INTEGRITY = "3GlxGdPvT2YHvQYOt+yOzT/DEior/qJVNIylHA77kgeoBTdQQqi70GwefNEUmHQeNGyjC8U62+bruqW/Tpm+tg=="
URL = f"https://registry.npmjs.org/hls.js/-/hls.js-{VERSION}.tgz"
DEST = Path(__file__).resolve().parents[1] / "frontend" / "vendor"

def main():
    with urllib.request.urlopen(URL, timeout=60) as response:
        data = response.read(16 * 1024 * 1024 + 1)
    if len(data) > 16 * 1024 * 1024:
        raise SystemExit("HLS package is too large")
    digest = base64.b64encode(hashlib.sha512(data).digest()).decode()
    if digest != INTEGRITY:
        raise SystemExit("HLS package integrity mismatch")
    DEST.mkdir(parents=True, exist_ok=True)
    with tarfile.open(fileobj=io.BytesIO(data), mode="r:gz") as archive:
        for src, dest in [("package/dist/hls.min.js", "hls.min.js"), ("package/LICENSE", "HLS-LICENSE.txt")]:
            member = archive.getmember(src)
            if not member.isfile() or member.size > 4 * 1024 * 1024:
                raise SystemExit("Unexpected HLS package member")
            with archive.extractfile(member) as content:
                (DEST / dest).write_bytes(content.read())
    print(f"Vendored HLS.js {VERSION}; npm SHA-512 verified")

if __name__ == "__main__":
    main()
