#!/bin/sh
# Install the certificate audit, or extend the NRG lineage after DNS is ready.
# This file must be root-owned before granting scoped sudo access to it.
set -eu

[ "$(id -u)" -eq 0 ] || { echo "root required" >&2; exit 1; }
[ "$#" -eq 1 ] || { echo "usage: $0 install|enroll" >&2; exit 2; }

stage=/home/xper626/cert-watch-stage
nginx_conf=/etc/nginx/conf.d/ssl.conf
lineage=/etc/letsencrypt/live/nrg-api-admin/fullchain.pem

install_verified() {
    source=$1 target=$2 mode=$3 expected=$4
    [ -f "$source" ] || { echo "missing staged file: $source" >&2; exit 1; }
    tmp=$(mktemp "${target}.XXXXXX")
    trap 'rm -f "$tmp"' EXIT HUP INT TERM
    install -o root -g root -m "$mode" "$source" "$tmp"
    actual=$(sha256sum "$tmp" | cut -d ' ' -f 1)
    [ "$actual" = "$expected" ] || { echo "staged file hash mismatch: $source" >&2; exit 1; }
    mv -f "$tmp" "$target"
    trap - EXIT HUP INT TERM
}

case "$1" in
    install)
        [ -f "$stage/healthchecks-url" ] || { echo "missing Healthchecks ping URL in $stage/healthchecks-url" >&2; exit 1; }
        install -d -o root -g root -m 0700 /etc/cert-watch
        url_tmp=$(mktemp /etc/cert-watch/healthchecks-url.XXXXXX)
        trap 'rm -f "$url_tmp"' EXIT HUP INT TERM
        install -o root -g root -m 0600 "$stage/healthchecks-url" "$url_tmp"
        python3 - "$url_tmp" <<'PY'
import re
import sys
from pathlib import Path
from urllib.parse import urlsplit

raw = Path(sys.argv[1]).read_text().strip()
url = urlsplit(raw)
if url.scheme != "https" or url.netloc != "hc-ping.com" or not re.fullmatch(r"/[0-9a-fA-F-]{36}", url.path) or url.query or url.fragment:
    raise SystemExit("invalid Healthchecks ping URL")
PY
        mv -f "$url_tmp" /etc/cert-watch/healthchecks-url
        trap - EXIT HUP INT TERM
        rm -f "$stage/healthchecks-url"
        install -d -o root -g xper626 -m 2750 /var/lib/cert-watch
        install -d -o root -g root -m 0755 /usr/local/libexec
        install_verified "$stage/cert-watch.py" /usr/local/libexec/cert-watch.py 0755 74fad01a76d5af2e36593f148f55c3498d2ae804ee471343396073f77d0c10fb
        install_verified "$stage/cert-watch-ping.py" /usr/local/libexec/cert-watch-ping.py 0755 9a280685b0631f106a7a2b881152cb7b5980e665369cd833ab906d47e655cdb7
        /usr/local/libexec/cert-watch-ping.py success
        install_verified "$stage/cert-watch.service" /etc/systemd/system/cert-watch.service 0644 02c515bb505450842eb219c7c66756ae21e50275e057e804879f0985c44fed65
        install_verified "$stage/cert-watch-failed.service" /etc/systemd/system/cert-watch-failed.service 0644 1132123ba64937c678922fd75a5415b09a8539e5bad6f1d6161e8ce0268b667e
        install_verified "$stage/cert-watch.timer" /etc/systemd/system/cert-watch.timer 0644 90db3fe57835acea1b72ce021a1a5729f86f5088e909684a9013cdf140df938f
        systemctl daemon-reload
        systemctl enable --now cert-watch.timer
        systemctl is-active --quiet certbot.timer || { echo "certbot.timer is not active" >&2; exit 1; }
        if systemctl start cert-watch.service; then
            echo "certificate audit passed"
        else
            echo "certificate audit found issues; see /var/lib/cert-watch/report.json and journalctl -u cert-watch.service" >&2
        fi
        systemctl is-active --quiet cert-watch.timer
        ;;
    enroll)
        for name in api admin lp landing; do
            domain="$name.nouveauricheglobalgroup.com"
            if ! getent ahostsv4 "$domain" | awk '{print $1}' | grep -Fxq 139.59.135.253; then
                echo "DNS for $domain is not 139.59.135.253" >&2
                exit 1
            fi
        done
        certbot certonly --nginx --non-interactive --agree-tos --expand \
            --cert-name nrg-api-admin \
            -d api.nouveauricheglobalgroup.com \
            -d admin.nouveauricheglobalgroup.com \
            -d lp.nouveauricheglobalgroup.com \
            -d landing.nouveauricheglobalgroup.com
        for name in api admin lp landing; do
            if ! openssl x509 -in "$lineage" -noout -checkhost "$name.nouveauricheglobalgroup.com" | grep -Fq 'does match certificate'; then
                echo "issued certificate lacks $name" >&2
                exit 1
            fi
        done
        backup=$(mktemp /var/backups/nrg-ssl-conf.pre-four-san.XXXXXX)
        cp -p "$nginx_conf" "$backup"
        python3 - "$nginx_conf" <<'PY'
import os
import pathlib
import sys

path = pathlib.Path(sys.argv[1])
text = path.read_text()
old_cert = "ssl_certificate /etc/ssl/fullchain.pem;"
old_key = "ssl_certificate_key /etc/ssl/STAR_nouveauricheglobalgroup_com.key;"
if text.count(old_cert) != 2 or text.count(old_key) != 2:
    raise SystemExit("expected exactly two remaining Sectigo nginx hosts; config untouched")
new = text.replace(old_cert, "ssl_certificate /etc/letsencrypt/live/nrg-api-admin/fullchain.pem;")
new = new.replace(old_key, "ssl_certificate_key /etc/letsencrypt/live/nrg-api-admin/privkey.pem;")
temporary = path.with_name(path.name + ".cert-watch-new")
temporary.write_text(new)
os.chmod(temporary, path.stat().st_mode & 0o777)
os.replace(temporary, path)
PY
        if ! nginx -t || ! systemctl reload nginx; then
            cp -p "$backup" "$nginx_conf"
            nginx -t && systemctl reload nginx
            echo "nginx change rolled back; backup: $backup" >&2
            exit 1
        fi
        for name in api admin lp landing; do
            domain="$name.nouveauricheglobalgroup.com"
            if ! echo | openssl s_client -connect 127.0.0.1:443 -servername "$domain" 2>/dev/null |
                openssl x509 -noout -checkhost "$domain" | grep -Fq 'does match certificate'; then
                cp -p "$backup" "$nginx_conf"
                nginx -t && systemctl reload nginx
                echo "served certificate check failed for $domain; nginx restored from $backup" >&2
                exit 1
            fi
        done
        echo "NRG four-name certificate active; nginx backup: $backup"
        systemctl start cert-watch.service || true
        ;;
    *) echo "usage: $0 install|enroll" >&2; exit 2 ;;
esac
