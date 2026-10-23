#!/usr/bin/env python3
"""Signal the certificate audit result to one Healthchecks.io check."""

import argparse
import re
import sys
import urllib.parse
import urllib.request
from pathlib import Path


URL_FILE = Path("/etc/cert-watch/healthchecks-url")


def check_url(raw):
    parsed = urllib.parse.urlsplit(raw.strip())
    if parsed.scheme != "https" or parsed.netloc != "hc-ping.com":
        raise ValueError("ping URL must use https://hc-ping.com")
    if parsed.query or parsed.fragment or parsed.username or parsed.password:
        raise ValueError("ping URL must not contain credentials, query, or fragment")
    if not re.fullmatch(r"/[0-9a-fA-F-]{36}", parsed.path):
        raise ValueError("ping URL must identify one UUID check")
    return urllib.parse.urlunsplit((parsed.scheme, parsed.netloc, parsed.path, "", ""))


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("status", choices=("success", "fail"))
    args = parser.parse_args()
    url = check_url(URL_FILE.read_text())
    if args.status == "fail":
        url += "/fail"
    request = urllib.request.Request(url, method="POST", headers={"User-Agent": "cert-watch/1"})
    with urllib.request.urlopen(request, timeout=8) as response:
        if response.status < 200 or response.status >= 300:
            raise RuntimeError(f"Healthchecks returned HTTP {response.status}")
    print(f"Healthchecks accepted certificate audit {args.status} signal")
    return 0


if __name__ == "__main__":
    try:
        sys.exit(main())
    except (OSError, ValueError, RuntimeError) as exc:
        print(f"Healthchecks signal failed: {exc}", file=sys.stderr)
        sys.exit(1)
