#!/usr/bin/env bash

script_dir="$(cd "$(dirname "$0")" && pwd)"
bash "$script_dir/deploy-router.sh"
exit_code=$?
echo
read -r -n 1 -p "Press any key to close..."
echo
exit "$exit_code"
