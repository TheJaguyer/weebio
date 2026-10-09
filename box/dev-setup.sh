#!/bin/sh
# Prepare a freshly imaged Raspberry Pi OS Lite (64-bit, trixie) for testing Weebio builds by hand.
# Run as your normal user (not root) from an SSH session:
#   curl -fsSL https://raw.githubusercontent.com/<you>/weebio/main/box/dev-setup.sh | sh -s <you>
# Then log out and back in once (for the new group memberships).
set -eu

REPO_OWNER="${1:?usage: dev-setup.sh <github-user>}"

echo "== packages"
sudo apt-get update
sudo apt-get install -y \
    libgtk-4-1 libadwaita-1-0 libwebkitgtk-6.0-4 libmpv2 libepoxy0 \
    nodejs ffmpeg \
    labwc seatd \
    pipewire pipewire-pulse wireplumber \
    zstd curl

echo "== display + input access without a desktop login"
sudo systemctl enable --now seatd
groups="video,render,input"
# Some seatd packages gate their socket on a _seatd group; Debian's uses video. Only add it if present.
getent group _seatd >/dev/null && groups="$groups,_seatd"
sudo usermod -aG "$groups" "$USER"

echo "== audio (per-user PipeWire services)"
systemctl --user enable --now pipewire pipewire-pulse wireplumber

echo "== helper scripts in $HOME"

# Install a release into /opt/weebio/releases/<ver> and point /opt/weebio/current at it.
cat > "$HOME/weebio-install.sh" <<EOF
#!/bin/sh
# Usage: weebio-install.sh 0.0.6-dev
set -eu
V="\$1"; REPO="$REPO_OWNER/weebio"; T="weebio-\$V-arm64.tar.zst"
tmp=\$(mktemp -d); cd "\$tmp"
curl -fLO "https://github.com/\$REPO/releases/download/v\$V/\$T"
curl -fLO "https://github.com/\$REPO/releases/download/v\$V/\$T.sha256"
echo "\$(cat "\$T.sha256")  \$T" | sha256sum -c
sudo mkdir -p "/opt/weebio/releases/\$V"
sudo tar -C "/opt/weebio/releases/\$V" --zstd -xf "\$T"
sudo ln -sfn "/opt/weebio/releases/\$V" /opt/weebio/current
cd / && rm -rf "\$tmp"
echo "active: \$(cat /opt/weebio/current/VERSION)  shell: \$(sha256sum /opt/weebio/current/bin/weebio-shell | cut -c1-16)"
EOF

# What labwc runs (labwc -s does not use a shell, so the logic lives in a file).
cat > "$HOME/weebio-session.sh" <<'EOF'
#!/bin/sh
sleep 1
exec /opt/weebio/current/bin/weebio > "$HOME/weebio.log" 2>&1
EOF

# Start a test run on the TV.  Usage: weebio-run.sh [hwdec]   e.g. weebio-run.sh drm
cat > "$HOME/weebio-run.sh" <<'EOF'
#!/bin/sh
export LIBSEAT_BACKEND=seatd RUST_LOG=debug
[ -n "${1:-}" ] && export WEEBIO_HWDEC="$1"
echo "running $(cat /opt/weebio/current/VERSION) hwdec=${WEEBIO_HWDEC:-launcher default (drm)}; log: ~/weebio.log; stop with: pkill labwc"
exec labwc -s "$HOME/weebio-session.sh"
EOF

# Record open file descriptors every 2s (run in a second SSH session).  Usage: weebio-fds.sh [name]
cat > "$HOME/weebio-fds.sh" <<'EOF'
#!/bin/sh
out="$HOME/fds-${1:-run}.log"
echo "logging to $out (Ctrl+C to stop)"
while sleep 2; do
    for p in $(pgrep -f 'bin/weebio-shell|WebKitWebProcess'); do
        echo "$(date +%T) $(cat /proc/$p/comm) total=$(ls /proc/$p/fd | wc -l) sync=$(ls -l /proc/$p/fd | grep -c sync_file) dmabuf=$(ls -l /proc/$p/fd | grep -c dmabuf)"
    done
done | tee "$out"
EOF

chmod +x "$HOME"/weebio-install.sh "$HOME"/weebio-session.sh "$HOME"/weebio-run.sh "$HOME"/weebio-fds.sh

echo
echo "Done. Log out and SSH back in once, then:"
echo "  ~/weebio-install.sh 0.0.6-dev"
echo "  ~/weebio-run.sh              # second session: ~/weebio-fds.sh fix"
