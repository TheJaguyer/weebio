# Setting up a Weebio box

From blank SD card to a box that's ready to hand to a friend. Takes about 15 minutes per box,
most of it waiting on downloads.

## What each box needs

- Raspberry Pi 5 (4 GB or more), official 27 W USB-C power supply, Active Cooler (or a fan case)
- microSD card, 32 GB or larger (A2-rated is faster)
- micro-HDMI → HDMI cable, plugged into **HDMI 0, the port next to the power socket**
- For setup only: your Wi-Fi, a PC, and ideally a USB keyboard

## 1. Flash the SD card

In **Raspberry Pi Imager**: Device **Raspberry Pi 5**, OS **Raspberry Pi OS Lite (64-bit)**
(the Debian *Trixie* based one, under "Raspberry Pi OS (other)"). Then edit the OS customisation settings:

| Setting | Value |
|---|---|
| Hostname | unique per box: `weebio-01`, `weebio-02`, … |
| Username / password | your admin account (same on every box is fine) |
| Wi-Fi | **your** network: only used during setup, removed before shipping (step 5) |
| Locale / time zone | the friend's time zone |
| Services → SSH | enabled; **public-key only** recommended (the box will sit on friends' networks) |

## 2. Provision

Boot the Pi, wait a minute, then from your PC:

```sh
ssh <you>@weebio-01.local
```

On the Pi, set `VERSION` to the newest release on
<https://github.com/TheJaguyer/weebio/releases> and run:

```sh
VERSION=0.1.1-dev
curl -fsSL https://raw.githubusercontent.com/TheJaguyer/weebio/v$VERSION/box/provision.sh | sh -s TheJaguyer $VERSION
```

The link uses the **release tag**, so the script always matches the release it installs.
It installs packages, creates the locked `weebio` kiosk user, installs the release, and sets the box
to boot into the kiosk. Every step is safe to re-run if something fails halfway.

## 3. Add the secrets file

API keys for addons go here, never in the repo. Names must match the `{{KEY}}` placeholders in `addons.json`:

```sh
sudo tee /etc/weebio/secrets.env >/dev/null <<'EOF'
RD_KEY=your-real-debrid-key
EOF
sudo chmod 600 /etc/weebio/secrets.env
```

An addon whose key is missing is skipped (not installed broken).

## 4. Reboot and check

```sh
sudo reboot
```

The TV should show the logo and spinner, then Stremio. Over SSH, confirm:

```sh
systemctl is-active weebio-agent weebio-kiosk     # both: active
cat /opt/weebio/current/VERSION                   # the version you installed
```

Play a stream to confirm playback.

## 5. Before shipping: remove your Wi-Fi

The box must forget your network so that at the friend's house it opens Wi-Fi setup straight away.
This cuts the SSH connection, so the command schedules a power-off first:

```sh
nmcli -t -f NAME,TYPE con show                      # find your Wi-Fi connection's name
sudo systemd-run --on-active=10s systemctl poweroff && sudo nmcli con delete "<that name>"
```

The box powers off ~10 seconds later. Label it with its hostname and record it below.

At the friend's house: plug in power + HDMI. Within a few seconds of booting, the box shows the
Wi-Fi setup page; once they pick their network and enter the password, it goes straight into Stremio.

## Updating a box

Until automatic updates exist, a box updates over SSH (so only while it's on a network you can reach):

```sh
sudo weebio-install <version>
```

This installs the release, refreshes the service files and restarts the kiosk.

## Troubleshooting

```sh
journalctl -b -u weebio-agent --no-pager | tail -30          # agent: UI server, Wi-Fi
journalctl -b -u weebio-kiosk --no-pager | tail -30          # labwc session
journalctl -b -t weebio-shell --no-pager | tail -50          # Stremio shell, player, streaming server
journalctl -b -t kanshi --no-pager | tail                    # display resolution
```

- **Black screen with a mouse cursor:** the shell didn't start or WebKit's sandbox refused to.
  Check the `weebio-shell` log for `bwrap` errors.
- **Wrong resolution or not fullscreen:** check the `kanshi` log; the TV should be on 1920×1080.
- **Preview the Wi-Fi page without deleting Wi-Fi:**
  `sudo systemctl set-environment WEEBIO_URL=http://127.0.0.1:8090/weebio/setup && sudo systemctl restart weebio-kiosk`
  (undo with `sudo systemctl unset-environment WEEBIO_URL && sudo systemctl restart weebio-kiosk`).

## Box log

| Box | Hostname | Friend | Version | Shipped |
|---|---|---|---|---|
| 1 | weebio-01 | | | |
| 2 | weebio-02 | | | |
| 3 | weebio-03 | | | |
| 4 | weebio-04 | | | |
| 5 | weebio-05 | | | |
| 6 | weebio-06 | | | |
| 7 | weebio-07 | | | |
| 8 | weebio-08 | | | |
| 9 | weebio-09 | | | |
| 10 | weebio-10 | | | |
