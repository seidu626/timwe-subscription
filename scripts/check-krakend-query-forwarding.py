#!/usr/bin/env python3
"""Validate KrakenD list endpoints forward query strings."""

from __future__ import annotations

import json
import sys
from pathlib import Path
from typing import Any


REQUIRED_ENDPOINTS = (
    "/api/v1/subscription/list",
    "/api/v1/notification/list",
)


def _load_config(config_path: Path) -> dict[str, Any]:
    try:
        return json.loads(config_path.read_text(encoding="utf-8"))
    except FileNotFoundError as exc:
        raise RuntimeError(f"config file not found: {config_path}") from exc
    except json.JSONDecodeError as exc:
        raise RuntimeError(f"invalid JSON in {config_path}: {exc}") from exc


def _validate(config: dict[str, Any], config_path: Path) -> list[str]:
    errors: list[str] = []
    endpoints = config.get("endpoints")
    if not isinstance(endpoints, list):
        return [f"{config_path}: missing or invalid 'endpoints' array"]

    endpoint_map = {
        endpoint.get("endpoint"): endpoint
        for endpoint in endpoints
        if isinstance(endpoint, dict) and isinstance(endpoint.get("endpoint"), str)
    }

    for route in REQUIRED_ENDPOINTS:
        endpoint = endpoint_map.get(route)
        if endpoint is None:
            errors.append(f"{config_path}: missing endpoint {route}")
            continue

        input_query_strings = endpoint.get("input_query_strings")
        if not isinstance(input_query_strings, list) or not all(
            isinstance(value, str) for value in input_query_strings
        ):
            errors.append(f"{config_path}: endpoint {route} has invalid input_query_strings")
            continue

        if "*" not in input_query_strings:
            errors.append(
                f"{config_path}: endpoint {route} must include '*' in input_query_strings"
            )

    return errors


def _validate_tenant_templates(repo_root: Path) -> list[str]:
    errors: list[str] = []
    tenant_template = repo_root / "krakend" / "config" / "templates" / "TenantApiEndpoint.tmpl"
    endpoint_template = repo_root / "krakend" / "config" / "templates" / "Endpoint.tmpl"

    tenant_source = tenant_template.read_text(encoding="utf-8")
    endpoint_source = endpoint_template.read_text(encoding="utf-8")

    for literal in ('"value": "{tenant_key}"', '"value": "{channel_key}"'):
        if literal in tenant_source:
            errors.append(
                f"{tenant_template}: tenant routing must not use static Martian header value {literal}"
            )

    if "modifier/martian" in tenant_source:
        errors.append(
            f"{tenant_template}: tenant routing must use KrakenD parameter forwarding, not Martian"
        )

    required_path_routes = (
        "/api/external/v1/{tenant_key}/{channel_key}/subscriptions/optin",
        "/api/external/v1/{tenant_key}/{channel_key}/subscriptions/confirm",
        "/api/external/v1/{tenant_key}/{channel_key}/subscriptions/optout",
        "/api/external/v1/{tenant_key}/{channel_key}/subscriptions/status",
    )
    for route in required_path_routes:
        route_line = next(
            (line for line in endpoint_source.splitlines() if route in line),
            "",
        )
        if "TenantPathApiEndpoint" not in route_line:
            errors.append(
                f"{endpoint_template}: route {route} must use TenantPathApiEndpoint"
            )

    return errors


def main() -> int:
    repo_root = Path(__file__).resolve().parents[1]
    config_path = (
        Path(sys.argv[1]).resolve()
        if len(sys.argv) > 1
        else repo_root / "krakend" / "krakend.json"
    )

    try:
        config = _load_config(config_path)
        errors = _validate(config, config_path)
        errors.extend(_validate_tenant_templates(repo_root))
    except RuntimeError as exc:
        print(f"ERROR: {exc}")
        return 1

    if errors:
        for error in errors:
            print(f"ERROR: {error}")
        return 1

    print(f"KrakenD query forwarding check passed: {config_path}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
