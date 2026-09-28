# oPKGui

A Package Center-style GUI for [Entware](https://github.com/Entware/Entware)'s `opkg` package
manager on Synology DSM — browse, install, remove, and upgrade Entware packages from a real
embedded DSM desktop window, instead of dropping to SSH for every `opkg install`.

Entware is the standard way to get software onto a Synology NAS that isn't packaged as a
proper DSM `.spk` (e.g. `tinyproxy`), but it's CLI-only. oPKGui doesn't replace or reimplement
`opkg` — it's a thin, careful GUI wrapper around the real thing.

**Status:** working end-to-end on real DSM 7 hardware — read-only browsing, install/remove/
upgrade, and a real embedded Package Center-style window are all implemented and tested live.
Currently self-hosted (manual `.spk` sideload); not yet distributed through SynoCommunity.
See [`DESIGN.md`](DESIGN.md) for the full round-by-round build history and
[`TESTING.md`](TESTING.md) for build/install/test steps.

## Requirements

- DSM 7
- [Entware already installed](https://github.com/Entware/Entware/wiki/Install-on-Synology-NAS)
  (oPKGui manages Entware's packages; it doesn't bootstrap Entware itself)

## Install

1. Build the `.spk` (see below) or grab one from a CI run's build artifacts.
2. DSM → **Package Center** → **Manual Install** → select the `.spk`. DSM will warn it's from
   an unidentified publisher (expected — unsigned); proceed anyway.
3. Open oPKGui from the Main Menu, click **Set Up Permission Grant Task**, then in
   **Control Panel → Task Scheduler**, select the new task and click **Run** once. This grants
   oPKGui's service account read access to Entware's config/status and a narrow `sudoers` rule
   scoped to exactly `opkg install`/`remove`/`upgrade` — the package itself never runs as root
   (DSM 7 blocks that for unsigned packages by design; see `DESIGN.md`'s Round 5).

Full walkthrough, troubleshooting, and what to expect at each step: [`TESTING.md`](TESTING.md).

## Build

Requires Docker and a local Go toolchain:

```sh
./build.sh
```

Builds through SynoCommunity's real [`spksrc`](https://github.com/SynoCommunity/spksrc)
framework via their published Docker image, rather than hand-assembling the `.spk` format.
Set `ARCH` (`x64`, `aarch64`, `armv7`, default `noarch`) and `TCVERSION` (default `7.1`) to
target a specific platform/DSM toolchain — see [`.github/workflows/build.yml`](.github/workflows/build.yml)
for the full supported matrix, built and verified on every push.

## How it works

A single static Go binary (own embedded HTTP+HTTPS server, self-signed TLS, no PHP/WebStation
dependency) that shells out to `opkg` with fixed, validated arguments only — never free-text
shell construction from user input. The DSM desktop window is real `SYNO.SDS.AppWindow`/
`AppInstance` ExtJS widgets under a `SYNOCOMMUNITY.OPKGui.*` namespace (the documented
SynoCommunity third-party app pattern), not an iframe or a new browser tab. Full architectural
reasoning and every dead end tried along the way is in [`DESIGN.md`](DESIGN.md).

## License

[GPL-2.0](LICENSE) — matching the license of [Entware](https://github.com/Entware/Entware),
which this project depends on.
