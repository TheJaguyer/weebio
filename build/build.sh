#!/usr/bin/env bash
# Build a Weebio arm64 release bundle into dist/.
#   build/build.sh [version]       (version defaults to `git describe`)
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
version="${1:-$(git -C "$root" describe --tags --always --dirty)}"
out="$root/dist"
stage="$out/weebio-$version"

if [[ ! -e /proc/sys/fs/binfmt_misc/qemu-aarch64 ]]; then
    echo "arm64 emulation is not registered. Run once (needs root via docker):" >&2
    echo "  docker run --privileged --rm tonistiigi/binfmt --install arm64" >&2
    exit 1
fi

for sub in stremio-web stremio-linux-shell; do
    [[ -e "$root/upstream/$sub/.git" ]] || { echo "missing upstream/$sub; run: git submodule update --init --recursive" >&2; exit 1; }
done
web_commit="$(git -C "$root/upstream/stremio-web" rev-parse HEAD)"

rm -rf "$stage"
mkdir -p "$stage"
docker buildx build \
    --file "$root/build/Dockerfile" \
    --target bundle \
    --build-arg "WEB_COMMIT=$web_commit" \
    --output "type=local,dest=$stage" \
    "$root"

echo "$version" > "$stage/VERSION"

tarball="$out/weebio-$version-arm64.tar.zst"
tar -C "$stage" -cf - . | zstd -19 -T0 -q -f -o "$tarball"
sha256sum "$tarball" | awk '{print $1}' > "$tarball.sha256"
echo "built $tarball"
