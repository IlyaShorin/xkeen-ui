#!/usr/bin/env bash

set -euo pipefail

default_host="192.168.1.1"
default_user="root"
default_repo_slug="IlyaShorin/xkeen-ui"
default_release_ref="latest"
panel_port="9081"
remote_binary="/opt/sbin/xkeen-ui"
remote_config="/opt/etc/xkeen-ui/config.yaml"
remote_init="/opt/etc/init.d/S26xkeen-ui"
restart_service="1"
overwrite_config="0"
host=""
user=""
local_config=""
repo_slug="$default_repo_slug"
release_ref="$default_release_ref"
generated_config_path=""
panel_username=""
panel_password=""

usage() {
  cat <<EOF
Usage: $(basename "$0") [options]

Options:
  --host <host>
  --user <user>
  --config <path>
  --repo <owner/name>
  --tag <tag|latest>
  --overwrite-config
  --no-restart
  --help
EOF
}

upload_file() {
  local source_path="$1"
  local target_path="$2"

  if scp -O "${ssh_options[@]}" "$source_path" "$ssh_target:$target_path" >/dev/null 2>&1; then
    return 0
  fi

  ssh "${ssh_options[@]}" "$ssh_target" "cat > '$target_path'" < "$source_path"
}

prompt_with_default() {
  local label="$1"
  local fallback="$2"
  local value=""

  read -r -p "$label [$fallback]: " value
  if [ -z "$value" ]; then
    value="$fallback"
  fi

  printf '%s' "$value"
}

prompt_yes_no() {
  local label="$1"
  local fallback="$2"
  local value=""

  read -r -p "$label [$fallback]: " value
  if [ -z "$value" ]; then
    value="$fallback"
  fi

  case "$value" in
    y|Y|yes|YES) printf '1' ;;
    n|N|no|NO) printf '0' ;;
    *) echo "Expected y or n" >&2; exit 1 ;;
  esac
}

prompt_secret() {
  local label="$1"
  local value=""
  local confirm=""

  while true; do
    read -r -s -p "$label: " value
    echo

    if [ -z "$value" ]; then
      echo "Value must not be empty." >&2
      continue
    fi

    read -r -s -p "Repeat $label: " confirm
    echo

    if [ "$value" != "$confirm" ]; then
      echo "Values do not match. Try again." >&2
      continue
    fi

    printf '%s' "$value"
    return 0
  done
}

hash_password() {
  local password="$1"

  if command -v python3 >/dev/null 2>&1; then
    python3 - "$password" <<'PY'
import hashlib
import secrets
import sys

password = sys.argv[1].encode()
iterations = 120000
salt = secrets.token_bytes(16)
digest = hashlib.sha256(salt + password).digest()
for _ in range(1, iterations):
    digest = hashlib.sha256(digest + salt + password).digest()
print(f"sha256${iterations}${salt.hex()}${digest.hex()}")
PY
    return 0
  fi

  echo "python3 is required to generate a panel password hash" >&2
  exit 1
}

cleanup_generated_config() {
  if [ -n "$generated_config_path" ] && [ -f "$generated_config_path" ]; then
    rm -f "$generated_config_path"
  fi
}

bundle_url_for() {
  local repo="$1"
  local ref="$2"
  local target="$3"
  local asset_name="xkeen-ui-${target}.tar.gz"

  if [ "$ref" = "latest" ]; then
    printf 'https://github.com/%s/releases/latest/download/%s' "$repo" "$asset_name"
    return 0
  fi

  printf 'https://github.com/%s/releases/download/%s/%s' "$repo" "$ref" "$asset_name"
}

resolve_config_source() {
  if [ -n "$local_config" ]; then
    if [ ! -f "$local_config" ]; then
      echo "Config file not found: $local_config" >&2
      exit 1
    fi
    return 0
  fi

  panel_username="$(prompt_with_default "Panel username" "$user")"
  panel_password="$(prompt_secret "Panel password")"
  generated_config_path="$(mktemp "${TMPDIR:-/tmp}/xkeen-ui-config.XXXXXX.yaml")"

  local password_hash
  password_hash="$(hash_password "$panel_password")"

  cat > "$generated_config_path" <<EOF
listen: 0.0.0.0:${panel_port}
username: ${panel_username}
password_hash: ${password_hash}
allow_cidrs:
  - 127.0.0.1/32
  - 192.168.0.0/16
  - 10.0.0.0/8
  - 172.16.0.0/12
xkeen_bin: /opt/sbin/xkeen
xray_bin: /opt/sbin/xray
xray_service: /opt/etc/init.d/S24xray
xray_config_dir: /opt/etc/xray/configs
backup_dir: /opt/backups/xkeen-ui
log_files:
  xkeen_ui_service: /opt/var/log/xkeen-ui/service.log
  xray_access: /opt/var/log/xray/access.log
  xray_error: /opt/var/log/xray/error.log
EOF

  local_config="$generated_config_path"
}

print_completion() {
  local panel_url="$1"
  local panel_username="$2"
  local panel_password="$3"
  local config_mode="$4"

  echo
  echo "Personal cabinet:"
  echo "  $panel_url"
  echo

  case "$config_mode" in
    generated)
      echo "Panel login: $panel_username"
      echo "Panel password: $panel_password"
      echo
      ;;
    uploaded)
      echo "Panel credentials come from: $local_config"
      echo
      ;;
    *)
      echo "Use the existing panel credentials from $remote_config"
      echo
      ;;
  esac

  if command -v open >/dev/null 2>&1; then
    open "$panel_url" >/dev/null 2>&1 || true
  elif command -v xdg-open >/dev/null 2>&1; then
    xdg-open "$panel_url" >/dev/null 2>&1 || true
  fi
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --host)
      host="${2:-}"
      shift 2
      ;;
    --user)
      user="${2:-}"
      shift 2
      ;;
    --config)
      local_config="${2:-}"
      shift 2
      ;;
    --repo)
      repo_slug="${2:-}"
      shift 2
      ;;
    --tag)
      release_ref="${2:-}"
      shift 2
      ;;
    --overwrite-config)
      overwrite_config="1"
      shift
      ;;
    --no-restart)
      restart_service="0"
      shift
      ;;
    --help|-h)
      usage
      exit 0
      ;;
    *)
      echo "Unknown option: $1" >&2
      usage
      exit 1
      ;;
  esac
done

if [ -z "$host" ]; then
  host="$(prompt_with_default "Router host" "$default_host")"
fi

if [ -z "$user" ]; then
  user="$(prompt_with_default "Router user" "$default_user")"
fi

ssh_target="$user@$host"
control_path="${TMPDIR:-/tmp}/xkeen-ui-ssh-$(echo "$ssh_target" | tr '@/:' '____')"
ssh_options=(
  -o ControlMaster=auto
  -o ControlPersist=10m
  -o ControlPath="$control_path"
  -o PreferredAuthentications=password
  -o PubkeyAuthentication=no
  -o KbdInteractiveAuthentication=no
  -o StrictHostKeyChecking=accept-new
)

cleanup() {
  ssh "${ssh_options[@]}" -O exit "$ssh_target" >/dev/null 2>&1 || true
  cleanup_generated_config
}

trap cleanup EXIT

echo "Opening SSH connection to $ssh_target"
ssh "${ssh_options[@]}" "$ssh_target" "true"

remote_probe="$(
  ssh "${ssh_options[@]}" "$ssh_target" '
    set -eu
    arch=""
    if command -v opkg >/dev/null 2>&1; then
      arch="$(opkg print-architecture 2>/dev/null | awk "{print \$2}" | grep -E "mipsel|mips|aarch64|arm64" | head -n 1 || true)"
    fi
    if [ -z "$arch" ] && command -v uname >/dev/null 2>&1; then
      arch="$(uname -m)"
    fi
    case "$arch" in
      *mipsel*|*mipsle*) target="linux-mipsle" ;;
      *mips*) target="linux-mips" ;;
      *aarch64*|*arm64*) target="linux-arm64" ;;
      *) echo "target=unsupported"; echo "arch=$arch"; echo "config_exists=0"; exit 0 ;;
    esac
    if [ -f /opt/etc/xkeen-ui/config.yaml ]; then
      config_exists=1
    else
      config_exists=0
    fi
    echo "target=$target"
    echo "arch=$arch"
    echo "config_exists=$config_exists"
  '
)"

target=""
remote_arch=""
config_exists="0"

while IFS='=' read -r key value; do
  case "$key" in
    target) target="$value" ;;
    arch) remote_arch="$value" ;;
    config_exists) config_exists="$value" ;;
  esac
done <<< "$remote_probe"

if [ "$target" = "unsupported" ] || [ -z "$target" ]; then
  echo "Unsupported router architecture: ${remote_arch:-unknown}" >&2
  exit 1
fi

if [ "$config_exists" = "0" ]; then
  overwrite_config="1"
fi

config_mode="existing"
if [ "$overwrite_config" = "1" ]; then
  resolve_config_source
  if [ -n "$generated_config_path" ]; then
    config_mode="generated"
  else
    config_mode="uploaded"
  fi
fi

bundle_url="$(bundle_url_for "$repo_slug" "$release_ref" "$target")"

if ! curl -fsIL "$bundle_url" >/dev/null 2>&1; then
  echo "Release asset is not available: $bundle_url" >&2
  echo "Push a tagged release first, for example: git tag v0.1.1 && git push origin main --tags" >&2
  exit 1
fi

remote_tmp="$(ssh "${ssh_options[@]}" "$ssh_target" 'mktemp -d /tmp/xkeen-ui-deploy.XXXXXX')"
asset_name="xkeen-ui-${target}.tar.gz"

echo "Downloading release bundle $bundle_url on the router"
ssh "${ssh_options[@]}" "$ssh_target" "
  set -eu
  remote_tmp='$remote_tmp'
  bundle_url='$bundle_url'
  bundle_path=\"\$remote_tmp/$asset_name\"

  if command -v curl >/dev/null 2>&1; then
    curl -fsSL --retry 3 --retry-delay 1 \"\$bundle_url\" -o \"\$bundle_path\"
  elif command -v wget >/dev/null 2>&1; then
    wget -qO \"\$bundle_path\" \"\$bundle_url\"
  else
    echo 'Neither curl nor wget is available on the router' >&2
    exit 1
  fi

  tar -xzf \"\$bundle_path\" -C \"\$remote_tmp\"
"

if [ "$overwrite_config" = "1" ]; then
  echo "Uploading config to $ssh_target:$remote_tmp/config.yaml"
  upload_file "$local_config" "$remote_tmp/config.yaml"
fi

echo "Installing xkeen-ui on the router"
ssh "${ssh_options[@]}" "$ssh_target" "
  set -eu
  remote_tmp='$remote_tmp'
  remote_binary='$remote_binary'
  remote_config='$remote_config'
  remote_init='$remote_init'
  overwrite_config='$overwrite_config'
  restart_service='$restart_service'
  backup_root='/opt/backups/xkeen-ui'
  timestamp=\$(date +%Y%m%d-%H%M%S 2>/dev/null || busybox date +%Y%m%d-%H%M%S)

  mkdir -p /opt/sbin /opt/etc/xkeen-ui /opt/etc/init.d /opt/var/log/xkeen-ui \"\$backup_root\"

  if [ -f \"\$remote_binary\" ]; then
    cp \"\$remote_binary\" \"\$backup_root/xkeen-ui.\$timestamp.bin\"
  fi

  cp \"\$remote_tmp/xkeen-ui\" \"\$remote_binary\"
  chmod 0755 \"\$remote_binary\"

  if [ -f \"\$remote_init\" ]; then
    cp \"\$remote_init\" \"\$backup_root/S26xkeen-ui.\$timestamp\"
  fi

  cp \"\$remote_tmp/S26xkeen-ui\" \"\$remote_init\"
  chmod 0755 \"\$remote_init\"

  if [ \"\$overwrite_config\" = '1' ]; then
    if [ -f \"\$remote_config\" ]; then
      cp \"\$remote_config\" \"\$backup_root/config.yaml.\$timestamp\"
    fi

    cp \"\$remote_tmp/config.yaml\" \"\$remote_config\"
    chmod 0644 \"\$remote_config\"
  fi

  rm -rf \"\$remote_tmp\"

  if [ \"\$restart_service\" = '1' ]; then
    \"\$remote_init\" restart >/dev/null 2>&1 || \"\$remote_init\" start
  fi

  echo 'binary='\"\$remote_binary\"
  echo 'config='\"\$remote_config\"
  echo 'init='\"\$remote_init\"
  echo 'bundle='\"$bundle_url\"
  \"\$remote_init\" status || true
"

panel_url="http://${host}:${panel_port}/"
print_completion "$panel_url" "$panel_username" "$panel_password" "$config_mode"

echo "Deployment complete"
