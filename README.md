# xkeen-ui

`xkeen-ui` is a small LAN-only web panel for routers running `xkeen` and `xray`.

It is designed for Entware/Keenetic deployments:

- no Node.js
- no SPA
- no external database
- one Go binary
- server-rendered HTML

## Features

- browse and edit files in `/opt/etc/xray/configs`
- create automatic backups before every save
- validate configs with `xray run -test -confdir ...`
- inspect merged config with `xray run -dump -confdir ...`
- run a safe whitelist of `xkeen` and `xray` actions
- read tail output from `xray` and `xkeen` logs

## Security model

- listen on LAN only
- Basic Auth
- CIDR allowlist
- no shell access
- no arbitrary file paths

## Password hashes

The service stays stdlib-only, so `hash-password` emits a salted iterative SHA-256 hash:

```text
sha256$120000$<salt-hex>$<digest-hex>
```

## Local development

```bash
make fmt
make test
make build
```

Generate a password hash:

```bash
./bin/xkeen-ui hash-password -password 'change-me'
```

## Build router binaries

```bash
make dist
```

For `mips` and `mipsle` targets the Makefile uses `go1.22.11`, because newer Go MIPS runtimes may crash on older Keenetic/Entware kernels.

Artifacts:

- `dist/linux-mipsle/xkeen-ui`
- `dist/linux-mips/xkeen-ui`
- `dist/linux-arm64/xkeen-ui`

## GitHub releases

Tagged releases publish router bundles to GitHub Releases through `.github/workflows/release.yml`.

Each release uploads:

- `xkeen-ui-linux-mipsle.tar.gz`
- `xkeen-ui-linux-mips.tar.gz`
- `xkeen-ui-linux-arm64.tar.gz`
- `SHA256SUMS.txt`

Create a release:

```bash
git tag v0.1.0
git push origin main --tags
```

## Router installation

1. Copy the binary to `/opt/sbin/xkeen-ui`.
2. Copy `config/config.example.yaml` to `/opt/etc/xkeen-ui/config.yaml` and edit it.
3. Copy `scripts/S26xkeen-ui` to `/opt/etc/init.d/S26xkeen-ui`.
4. Make both files executable where needed.
5. Start the service:

```bash
/opt/etc/init.d/S26xkeen-ui start
```

The default UI address is:

```text
http://<router-ip>:9081
```

## Interactive deploy script

There is also an interactive deploy helper:

```bash
./scripts/deploy-router.sh
```

On macOS there is also a double-clickable launcher:

```bash
./scripts/deploy-router.command
```

It:

- asks for router host and user
- downloads the correct release bundle from GitHub Releases directly on the router
- detects the router architecture
- installs the binary and init script from the release bundle
- generates a panel config on first install if the router does not have one yet
- optionally uploads a local config file when `--config` and `--overwrite-config` are used
- backs up existing remote files before overwrite
- restarts the service by default
- prints the panel URL and opens it automatically on macOS/Linux desktops

Useful options:

```bash
./scripts/deploy-router.sh
./scripts/deploy-router.sh --tag v0.1.0
./scripts/deploy-router.sh --config ./my-router-config.yaml --overwrite-config
```
