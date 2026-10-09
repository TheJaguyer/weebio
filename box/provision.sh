#!/bin/sh
# Turn a freshly imaged Raspberry Pi OS Lite (64-bit, trixie) into a Weebio box that boots straight
# into Stremio (or Wi-Fi setup). Run from an SSH session as the admin user, then reboot:
#   curl -fsSL https://raw.githubusercontent.com/<you>/weebio/v<version>/box/provision.sh | sh -s <you> <version>
# (see setup-instructions.md for the full per-box procedure)
set -eu

REPO_OWNER="${1:?usage: provision.sh <github-user> <version>}"
VERSION="${2:?usage: provision.sh <github-user> <version>}"

echo "== packages"
sudo apt-get update
sudo apt-get install -y \
    libgtk-4-1 libadwaita-1-0 libwebkitgtk-6.0-4 libmpv2 libepoxy0 \
    nodejs ffmpeg \
    labwc kanshi \
    pipewire pipewire-pulse wireplumber \
    network-manager \
    zstd curl

echo "== kiosk user (no password: it can't log in, it only runs the TV session)"
if ! id weebio >/dev/null 2>&1; then
    sudo useradd --create-home --shell /bin/bash weebio
fi
sudo usermod -aG video,render,input,audio weebio
sudo passwd -l weebio >/dev/null
# Audio for every user session, including the kiosk's.
sudo systemctl --global enable pipewire.socket pipewire-pulse.socket wireplumber.service

echo "== secrets directory (copy secrets.env here before shipping; root-only)"
sudo install -d -m 0700 -o root -g root /etc/weebio

echo "== release installer"
sudo tee /usr/local/sbin/weebio-install >/dev/null <<EOF
#!/bin/sh
# Install a Weebio release and make it active.  Usage: sudo weebio-install <version>
set -eu
V="\${1:?usage: weebio-install <version>}"; REPO="$REPO_OWNER/weebio"; T="weebio-\$V-arm64.tar.zst"
tmp=\$(mktemp -d); trap 'rm -rf "\$tmp"' EXIT; cd "\$tmp"
curl -fLO "https://github.com/\$REPO/releases/download/v\$V/\$T"
curl -fLO "https://github.com/\$REPO/releases/download/v\$V/\$T.sha256"
echo "\$(cat "\$T.sha256")  \$T" | sha256sum -c
mkdir -p "/opt/weebio/releases/\$V"
tar -C "/opt/weebio/releases/\$V" --zstd -xf "\$T"
ln -sfn "/opt/weebio/releases/\$V" /opt/weebio/current
# Service files ship inside each release so updates can change them.
install -m 0644 /opt/weebio/current/systemd/*.service /etc/systemd/system/
systemctl daemon-reload
systemctl try-restart weebio-agent.service weebio-kiosk.service
echo "active: \$(cat /opt/weebio/current/VERSION)"
EOF
sudo chmod 0755 /usr/local/sbin/weebio-install

echo "== first release"
sudo /usr/local/sbin/weebio-install "$VERSION"

echo "== boot into the kiosk"
sudo systemctl enable weebio-agent.service weebio-kiosk.service
sudo systemctl set-default multi-user.target

echo
echo "Done. Reboot to start the kiosk:  sudo reboot"
echo "Logs:  journalctl -u weebio-agent -u weebio-kiosk -t weebio-shell -f"
