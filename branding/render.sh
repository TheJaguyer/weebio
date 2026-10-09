#!/bin/sh
# Render branding/*.svg into every image the apps need. Runs in the Docker build (build/Dockerfile).
#   render.sh <branding-dir> <out-dir>
# Produces:
#   <out>/web/assets/images/*.png, <out>/web/assets/favicons/favicon.ico   (overrides stremio-web's art)
#   <out>/brand/{brand.json,icon.png,logo.png}                             (served by weebio-agent)
set -eu
src="$1"; out="$2"
img="$out/web/assets/images"; fav="$out/web/assets/favicons"; brand="$out/brand"
mkdir -p "$img" "$fav" "$brand"
bg=$(sed -n 's/.*"backgroundColor": *"\([^"]*\)".*/\1/p' "$src/brand.json")

icon() { rsvg-convert -w "$1" -h "$1" "$src/icon.svg" -o "$2"; }

icon 512 "$img/icon.png"
icon 512 "$img/icon_512x512.png"
icon 196 "$img/icon_196x196.png"
icon 256 "$img/stremio_symbol.png"          # the in-app symbol (nav bar, buffering)
rsvg-convert -w 670 -h 195 "$src/logo.svg" -o "$img/logo.png"

# Maskable icons: mark at 75% inside a solid background (Android/PWA safe zone).
for s in 512 196; do
    inner=$((s * 3 / 4))
    icon "$inner" "$out/inner.png"
    convert -size "${s}x${s}" "xc:$bg" "$out/inner.png" -gravity center -composite "$img/maskable_icon_${s}x${s}.png"
done
cp "$img/maskable_icon_512x512.png" "$img/maskable_icon.png"
convert "$img/icon_512x512.png" -fill white -colorize 100 "$img/monochrome_icon_512x512.png"

icon 16 "$out/f16.png"; icon 32 "$out/f32.png"; icon 48 "$out/f48.png"
convert "$out/f16.png" "$out/f32.png" "$out/f48.png" "$fav/favicon.ico"
rm -f "$out"/*.png

cp "$src/brand.json" "$brand/"
cp "$img/icon.png" "$img/logo.png" "$brand/"
