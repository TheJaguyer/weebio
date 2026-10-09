# Notice: what's in a Saga box, and under which licence

Saga is built from several pieces with different licences. The MIT licence in [`LICENSE`](LICENSE)
covers **only the code written for this project**; the other components keep their own licences.

| Component | Where | Licence |
|---|---|---|
| Weebio agent, box scripts, build tooling, branding, themes | this repository (`agent/`, `box/`, `build/`, `branding/`, `themes/`) | MIT, © 2026 Hof |
| Saga web UI: a modified [stremio-web](https://github.com/Stremio/stremio-web) | `upstream/stremio-web` (fork, branch `weebio`) | GNU GPL v2.0; modifications under the same licence |
| Saga shell: a modified [stremio-linux-shell](https://github.com/Stremio/stremio-linux-shell) | `upstream/stremio-linux-shell` (fork, branch `weebio`) | GNU GPL v3.0; modifications under the same licence |
| Streaming server (`server.js`) | shipped unmodified inside each release, as in stremio-linux-shell | Proprietary freeware by Smart Code Ltd (Stremio); redistributed unmodified |
| Third-party libraries | pulled in by the components above | their own licences (see each project) |

## Source code

The complete source for every release, including the GPL-licensed forks, is public:
<https://github.com/TheJaguyer/weebio> (submodules point at the exact fork commits used).
Each box shows this address in its credits, which satisfies the GPL's requirement that people
who receive the software can get its source.

## Trademarks

"Stremio" is a trademark of Smart Code Ltd. Saga is an independent, unofficial rebuild; it is not
affiliated with or endorsed by Stremio. The Stremio name and logos are replaced throughout the UI.
