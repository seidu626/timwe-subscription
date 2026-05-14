#!/usr/bin/env bash
set -euo pipefail

# The HE-aware Nouveau Riche Global server blocks are owned by:
#   /etc/nginx/conf.d/ssl.conf
#
# Older site-specific symlinks duplicate the same server_name values and cause
# nginx to ignore one of the competing blocks. Keep unrelated sites-enabled
# entries, such as veritasquest.io, untouched.

legacy_sites=(
  admin.nouveauricheglobalgroup.com
  api.nouveauricheglobalgroup.com
  landing.nouveauricheglobalgroup.com
)

sudo mkdir -p /etc/nginx/sites-disabled

for site in "${legacy_sites[@]}"; do
  enabled_path="/etc/nginx/sites-enabled/${site}"
  disabled_path="/etc/nginx/sites-disabled/${site}"

  if [[ -L "$enabled_path" ]]; then
    if [[ -e "$disabled_path" || -L "$disabled_path" ]]; then
      sudo rm -f "$disabled_path"
    fi
    sudo mv "$enabled_path" "$disabled_path"
    echo "disabled ${site}"
  elif [[ -L "$disabled_path" ]]; then
    echo "already disabled ${site}"
  else
    echo "not present ${site}"
  fi
done

sudo /usr/sbin/nginx -t
sudo systemctl reload nginx
