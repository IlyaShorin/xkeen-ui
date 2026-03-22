`deploy-router.sh` asks for router host and user, opens an interactive SSH session, detects the router architecture, downloads the matching GitHub Release bundle directly on the router, installs it into `/opt`, and restarts `xkeen-ui`.

Examples:

```bash
./scripts/deploy-router.sh
./scripts/deploy-router.sh --host 192.168.1.1 --user root
./scripts/deploy-router.sh --repo IlyaShorin/xkeen-ui --tag latest
./scripts/deploy-router.sh --config ./config/config.example.yaml --overwrite-config --no-restart
```
