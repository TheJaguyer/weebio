# syntax=docker/dockerfile:1.7
# Builds a Weebio release bundle for Raspberry Pi 5 (arm64, Pi OS / Debian trixie).
# Invoke via build/build.sh — it passes the repo root as the build context.

ARG DEBIAN_SUITE=trixie

# ---- stremio-web: architecture independent, so build natively (fast) ----
FROM --platform=$BUILDPLATFORM node:22-bookworm AS web
RUN corepack enable
WORKDIR /src
ARG WEB_COMMIT
ENV COMMIT_HASH=$WEB_COMMIT
COPY upstream/stremio-web/ ./
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

# ---- assemble the bundle layout that lands in /opt/weebio/releases/<ver> ----
FROM scratch AS bundle
COPY --from=shell /stremio-linux-shell                     /bin/weebio-shell
COPY --from=shell /src/data/server.js                      /lib/server.js
COPY --from=shell /schemas/                                /share/glib-2.0/schemas/
COPY box/weebio                                            /bin/weebio
COPY --from=web   /src/build/                              /web/
COPY themes/                                               /themes/
COPY addons.json                                           /addons.json
