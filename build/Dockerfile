# syntax=docker/dockerfile:1.7
# Builds a Weebio release bundle for Raspberry Pi 5 (arm64, Pi OS / Debian trixie).
# Invoke via build/build.sh — it passes the repo root as the build context.

ARG DEBIAN_SUITE=trixie

# ---- branding: render branding/*.svg into every image size the apps need ----
FROM --platform=$BUILDPLATFORM debian:${DEBIAN_SUITE} AS brand
RUN apt-get update && apt-get install -y --no-install-recommends librsvg2-bin imagemagick fonts-dejavu-core \
    && rm -rf /var/lib/apt/lists/*
COPY branding/ /branding/
RUN sh /branding/render.sh /branding /out

# ---- stremio-web: architecture independent, so build natively (fast) ----
FROM --platform=$BUILDPLATFORM node:22-bookworm AS web
RUN corepack enable
WORKDIR /src
ARG WEB_COMMIT
ENV COMMIT_HASH=$WEB_COMMIT
COPY upstream/stremio-web/ ./
# Saga branding: name/colours/credits, and our art over Stremio's logos, icons and favicon.
COPY branding/brand.json ./brand.json
COPY --from=brand /out/web/assets/ ./assets/
RUN --mount=type=cache,target=/root/.local/share/pnpm/store \
    pnpm install --frozen-lockfile && pnpm run build

# ---- stremio-linux-shell: native arm64 code, built under QEMU ----
FROM --platform=linux/arm64 debian:${DEBIAN_SUITE} AS shell
RUN apt-get update && apt-get install -y --no-install-recommends \
        build-essential pkg-config ca-certificates curl gettext \
        libgtk-4-dev libadwaita-1-dev libwebkitgtk-6.0-dev libmpv-dev libepoxy-dev \
    && rm -rf /var/lib/apt/lists/*
# Debian's rustc lags behind what the shell needs (edition 2024, let-chains).
RUN curl -sSf https://sh.rustup.rs | sh -s -- -y --profile minimal --default-toolchain stable
ENV PATH=/root/.cargo/bin:$PATH
WORKDIR /src
COPY upstream/stremio-linux-shell/ ./
RUN --mount=type=cache,target=/root/.cargo/registry \
    --mount=type=cache,target=/src/target,id=shell-target-arm64 \
    cargo build --release --locked && cp target/release/stremio-linux-shell /stremio-linux-shell
# build.rs installs the GSettings schema into the *build* user's home; ship it in the bundle instead.
RUN mkdir -p /schemas && cp data/com.stremio.Stremio.gschema.xml /schemas/ && glib-compile-schemas /schemas

# ---- weebio-agent: pure Go, tested natively then cross-compiled for arm64 ----
FROM --platform=$BUILDPLATFORM golang:1.24-bookworm AS agent
WORKDIR /src
COPY agent/ ./
RUN --mount=type=cache,target=/root/.cache/go-build \
    go vet ./... && go test ./... && \
    CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags="-s -w" -o /weebio-agent .

# ---- assemble the bundle layout that lands in /opt/weebio/releases/<ver> ----
FROM scratch AS bundle
COPY --from=shell /stremio-linux-shell                     /bin/weebio-shell
COPY --from=shell /src/data/server.js                      /lib/server.js
COPY --from=shell /schemas/                                /share/glib-2.0/schemas/
COPY box/weebio                                            /bin/weebio
COPY box/weebio-session                                    /bin/weebio-session
COPY --from=agent /weebio-agent                            /bin/weebio-agent
COPY box/kanshi.conf                                       /share/kanshi/config
COPY box/systemd/                                          /systemd/
COPY --from=web   /src/build/                              /web/
COPY themes/                                               /themes/
COPY --from=brand /out/brand/                              /branding/
COPY LICENSE NOTICE.md                                     /
COPY addons.json                                           /addons.json
