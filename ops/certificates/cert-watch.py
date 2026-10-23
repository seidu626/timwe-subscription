#!/usr/bin/env python3
"""Audit certificates used by nginx, Certbot, and listening TLS services.

Certbot's existing systemd timer performs renewal. This checker detects
unmanaged certificates, failures to renew, wrong names, and stale served certs.
"""

import argparse
import concurrent.futures
import datetime as dt
import json
import os
import re
import socket
import ssl
import subprocess
import sys
from pathlib import Path


UTC = dt.timezone.utc
LE_LIVE = Path("/etc/letsencrypt/live")
LE_RENEWAL = Path("/etc/letsencrypt/renewal")
REPORT = Path("/var/lib/cert-watch/report.json")
WARNING_DAYS = 30
CRITICAL_DAYS = 14


def run(argv, *, data=None, timeout=15):
    return subprocess.run(argv, input=data, capture_output=True, timeout=timeout, check=False)


def describe_cert(data, *, der=False, hostname=None):
    argv = ["openssl", "x509", "-inform", "DER" if der else "PEM", "-noout", "-startdate", "-enddate", "-subject", "-issuer", "-fingerprint", "-sha256"]
    proc = run(argv, data=data)
    if proc.returncode:
        raise ValueError("OpenSSL could not parse certificate")
    fields = dict(line.split("=", 1) for line in proc.stdout.decode().splitlines() if "=" in line)
    start = dt.datetime.strptime(fields["notBefore"], "%b %d %H:%M:%S %Y %Z").replace(tzinfo=UTC)
    expiry = dt.datetime.strptime(fields["notAfter"], "%b %d %H:%M:%S %Y %Z").replace(tzinfo=UTC)
    now = dt.datetime.now(UTC)
    result = {
        "expires_at": expiry.isoformat(),
        "valid_from": start.isoformat(),
        "validity_days": round((expiry - start).total_seconds() / 86400, 2),
        "days_remaining": round((expiry - now).total_seconds() / 86400, 2),
        "subject": fields.get("subject", ""),
        "issuer": fields.get("issuer", ""),
        "fingerprint_sha256": fields.get("sha256 Fingerprint", ""),
    }
    if hostname:
        check = run(["openssl", "x509", "-inform", "DER" if der else "PEM", "-noout", "-checkhost", hostname], data=data)
        result["hostname_matches"] = check.returncode == 0 and b"does match certificate" in check.stdout
    return result


def nginx_servers():
    proc = run(["/usr/sbin/nginx", "-T"], timeout=20)
    if proc.returncode:
        raise RuntimeError("nginx -T failed: " + proc.stderr.decode(errors="replace")[-500:])
    servers = []
    block = []
    depth = 0
    for raw in proc.stdout.decode(errors="replace").splitlines():
        line = raw.split("#", 1)[0].strip()
        if not block and re.match(r"^server\s*\{", line):
            block = [line]
            depth = line.count("{") - line.count("}")
        elif block:
            block.append(line)
            depth += line.count("{") - line.count("}")
        if block and depth == 0:
            content = "\n".join(block)
            names = re.findall(r"(?m)^server_name\s+([^;]+);", content)
            paths = re.findall(r"(?m)^ssl_certificate\s+([^;]+);", content)
            for path in paths:
                servers.append({"names": [name for group in names for name in group.split() if not name.startswith("$")], "path": path})
            block = []
    return servers


def certbot_lineages():
    names = set()
    if LE_RENEWAL.exists():
        names = {p.stem for p in LE_RENEWAL.glob("*.conf")}
    return names


def listening_targets():
    proc = run(["ss", "-ltnH"], timeout=10)
    if proc.returncode:
        raise RuntimeError("ss -ltnH failed")
    ports = set()
    for line in proc.stdout.decode().splitlines():
        parts = line.split()
        if len(parts) < 4:
            continue
        match = re.search(r":(\d+)$", parts[3])
        if match:
            ports.add(int(match.group(1)))
    return [("127.0.0.1", port, "host") for port in ports if port not in (22, 53, 80, 5355)]


def docker_targets():
    ids = run(["docker", "ps", "-q"], timeout=10)
    if ids.returncode:
        return [], "docker ps failed"
    targets = []
    for container_id in ids.stdout.decode().splitlines():
        # The format selects only network IPs and exposed ports; no Env or secrets.
        fmt = "{{range $p,$_ := .Config.ExposedPorts}}{{$p}} {{end}}|{{range .NetworkSettings.Networks}}{{.IPAddress}} {{end}}"
        proc = run(["docker", "inspect", "--format", fmt, container_id], timeout=5)
        if proc.returncode:
            continue
        ports_part, _, ips_part = proc.stdout.decode().partition("|")
        ports = [int(p.split("/")[0]) for p in ports_part.split() if p.endswith("/tcp")]
        for ip in ips_part.split():
            if ":" in ip:
                continue
            for port in ports:
                targets.append((ip, port, "docker:" + container_id[:12]))
    return targets, None


def tls_probe(target):
    host, port, source = target
    try:
        raw = socket.create_connection((host, port), timeout=1.5)
        raw.settimeout(1.5)
        context = ssl.SSLContext(ssl.PROTOCOL_TLS_CLIENT)
        context.check_hostname = False
        context.verify_mode = ssl.CERT_NONE
        with context.wrap_socket(raw, server_hostname="localhost") as conn:
            der = conn.getpeercert(binary_form=True)
        if not der:
            return None
        cert = describe_cert(der, der=True)
        cert.update({"source": source, "endpoint": f"{host}:{port}", "managed_by_certbot": False})
        return cert
    except (OSError, ssl.SSLError, ValueError, subprocess.TimeoutExpired):
        return None


def nginx_served_cert(hostname):
    raw = socket.create_connection(("127.0.0.1", 443), timeout=2)
    raw.settimeout(2)
    context = ssl.SSLContext(ssl.PROTOCOL_TLS_CLIENT)
    context.check_hostname = False
    context.verify_mode = ssl.CERT_NONE
    with context.wrap_socket(raw, server_hostname=hostname) as conn:
        der = conn.getpeercert(binary_form=True)
    return describe_cert(der, der=True, hostname=hostname)


def starttls_probe(host, port, protocol, source):
    proc = run(["openssl", "s_client", "-starttls", protocol, "-connect", f"{host}:{port}", "-showcerts"], data=b"", timeout=5)
    match = re.search(rb"-----BEGIN CERTIFICATE-----.*?-----END CERTIFICATE-----", proc.stdout, re.S)
    if not match:
        return None
    cert = describe_cert(match.group(), der=False)
    cert.update({"source": f"starttls:{protocol}", "endpoint": f"{host}:{port}", "listener_source": source, "managed_by_certbot": False})
    return cert


def exim_uses_generated_certificate():
    proc = run(["/usr/sbin/exim4", "-bP", "tls_certificate"], timeout=5)
    return proc.returncode == 0 and proc.stdout.decode().strip() == "tls_certificate ="


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--report", type=Path, default=REPORT)
    args = parser.parse_args()
    issues = []
    entries = []
    lineages = certbot_lineages()
    for name in sorted(lineages):
        path = LE_LIVE / name / "fullchain.pem"
        try:
            cert = describe_cert(path.read_bytes())
            cert.update({"source": "certbot", "path": str(path), "managed_by_certbot": True})
            entries.append(cert)
        except (OSError, ValueError, KeyError) as exc:
            issues.append(f"Certbot lineage {name}: {exc}")
    try:
        servers = nginx_servers()
        if not servers:
            issues.append("No nginx TLS server blocks discovered")
        for server in servers:
            path = server["path"]
            if "$" in path:
                issues.append(f"Dynamic nginx certificate path needs manual review: {path}")
                continue
            try:
                raw = Path(path).read_bytes()
                for name in server["names"] or [None]:
                    if name and ("~" in name or "*" in name):
                        continue
                    cert = describe_cert(raw, hostname=name)
                    cert.update({"source": "nginx", "path": path, "hostname": name, "managed_by_certbot": path.startswith(str(LE_LIVE) + "/") and Path(path).parts[4] in lineages})
                    entries.append(cert)
                    if name and not cert["hostname_matches"]:
                        issues.append(f"nginx {name}: certificate name mismatch")
                    if name and "." in name:
                        try:
                            socket.getaddrinfo(name, 443, type=socket.SOCK_STREAM)
                        except socket.gaierror:
                            issues.append(f"nginx {name}: DNS does not resolve")
                        try:
                            served = nginx_served_cert(name)
                            if served["fingerprint_sha256"] != cert["fingerprint_sha256"]:
                                issues.append(f"nginx {name}: served certificate differs from configured file")
                            if not served["hostname_matches"]:
                                issues.append(f"nginx {name}: served certificate name mismatch")
                        except (OSError, ssl.SSLError, ValueError, subprocess.TimeoutExpired):
                            issues.append(f"nginx {name}: local TLS probe failed")
            except (OSError, ValueError, KeyError) as exc:
                issues.append(f"nginx {path}: {exc}")
    except RuntimeError as exc:
        issues.append(str(exc))
    try:
        targets = listening_targets()
        docker, docker_error = docker_targets()
        if docker_error:
            issues.append(docker_error)
        targets = sorted(set(targets + docker))
        if len(targets) > 300:
            issues.append(f"TLS target discovery exceeded 300 endpoints ({len(targets)})")
            targets = targets[:300]
        with concurrent.futures.ThreadPoolExecutor(max_workers=16) as pool:
            for cert in pool.map(tls_probe, targets):
                if cert:
                    entries.append(cert)
        for host, port, source in targets:
            protocol = "smtp" if source == "host" and port == 25 else "postgres" if port == 5432 else None
            if protocol:
                try:
                    cert = starttls_probe(host, port, protocol, source)
                    if cert:
                        entries.append(cert)
                except (ValueError, KeyError, subprocess.TimeoutExpired):
                    issues.append(f"STARTTLS probe failed for {protocol} on {host}:{port}")
    except RuntimeError as exc:
        issues.append(str(exc))
    fingerprints = {cert["fingerprint_sha256"] for cert in entries if cert["source"] == "certbot"}
    exim_generated = exim_uses_generated_certificate() if any(cert["source"] == "starttls:smtp" for cert in entries) else False
    for cert in entries:
        if "endpoint" in cert:
            cert["managed_by_certbot"] = cert["fingerprint_sha256"] in fingerprints
        cert["managed_by_runtime"] = bool(exim_generated and cert["source"] == "starttls:smtp" and cert["validity_days"] <= 1)
    for cert in entries:
        days = cert["days_remaining"]
        if days < 0:
            issues.append(f"EXPIRED {cert['source']} {cert.get('hostname') or cert.get('endpoint') or cert.get('path')}")
        elif days <= WARNING_DAYS:
            label = cert.get("hostname") or cert.get("endpoint") or cert.get("path")
            if (not cert["managed_by_certbot"] and not cert["managed_by_runtime"]) or (cert["managed_by_certbot"] and days <= CRITICAL_DAYS):
                issues.append(f"RENEWAL NEEDED {cert['source']} {label}: {days} days remaining")
    report = {"checked_at": dt.datetime.now(UTC).isoformat(), "entries": entries, "issues": sorted(set(issues)), "warning_days": WARNING_DAYS, "critical_days": CRITICAL_DAYS}
    args.report.parent.mkdir(parents=True, exist_ok=True)
    temporary = args.report.with_suffix(".tmp")
    temporary.write_text(json.dumps(report, indent=2, sort_keys=True) + "\n")
    os.chmod(temporary, 0o640)
    temporary.replace(args.report)
    print(json.dumps({"checked_at": report["checked_at"], "certificates": len(entries), "issues": report["issues"]}))
    return 1 if issues else 0


if __name__ == "__main__":
    sys.exit(main())
