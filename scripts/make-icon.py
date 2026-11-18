#!/usr/bin/env python3
"""Create macOS and Windows icons from the user-supplied image."""
from pathlib import Path
import subprocess
from PIL import Image, ImageDraw, ImageOps

root = Path(__file__).resolve().parent.parent
source = root / "packaging/icon-source.jpg"
image = Image.open(source).convert("RGB")
image = ImageOps.fit(image, (1024, 1024), Image.Resampling.LANCZOS, centering=(0.5, 0.5)).convert("RGBA")
mask = Image.new("L", image.size, 0)
ImageDraw.Draw(mask).rounded_rectangle((0, 0, 1023, 1023), radius=220, fill=255)
image.putalpha(mask)
image.save(root / "packaging/app-icon.png")
image.save(root / "packaging/AppIcon.ico", format="ICO", sizes=[(n, n) for n in (16, 24, 32, 48, 64, 128, 256)])
image.resize((64, 64), Image.Resampling.LANCZOS).save(root / "packaging/sidebar-icon.png")
iconset = root / "build/AppIcon.iconset"
iconset.mkdir(parents=True, exist_ok=True)
for size in (16, 32, 128, 256, 512):
    for scale in (1, 2):
        suffix = "@2x" if scale == 2 else ""
        image.resize((size * scale, size * scale), Image.Resampling.LANCZOS).save(iconset / f"icon_{size}x{size}{suffix}.png")
subprocess.run(["iconutil", "-c", "icns", str(iconset), "-o", str(root / "packaging/AppIcon.icns")], check=True)
print("Created packaging/AppIcon.icns from the supplied image")
