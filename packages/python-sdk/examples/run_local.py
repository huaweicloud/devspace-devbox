"""Run an example with an optional process-local Gateway DNS override."""

from __future__ import annotations

import argparse
import os
import runpy
import socket
from fnmatch import fnmatchcase
from ipaddress import ip_address
from pathlib import Path
from typing import Any
from urllib.parse import urlsplit


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("example", choices=("basic", "async_basic", "demo", "validate_full"))
    args = parser.parse_args()
    original = socket.getaddrinfo
    gateway_ip = os.getenv("DEVBOX_GATEWAY_IP", "").strip()
    if gateway_ip:
        gateway_ip = str(ip_address(gateway_ip))
        hostname = urlsplit(os.getenv("DEVBOX_GATEWAY_URL", "")).hostname
        if not hostname:
            parser.error("DEVBOX_GATEWAY_IP requires DEVBOX_GATEWAY_URL")
        pattern = hostname.replace("{tunnel_id}", "*").replace("{port}", "49983")

        def resolve(
            host: bytes | str | None,
            port: bytes | str | int | None,
            *options: Any,
            **kwargs: Any,
        ) -> Any:
            name = host.decode("ascii") if isinstance(host, bytes) else host
            if name and fnmatchcase(name.lower(), pattern):
                host = gateway_ip
            return original(host, port, *options, **kwargs)

        # Only DNS changes; the request URL, HTTP Host and TLS SNI stay intact.
        socket.getaddrinfo = resolve
        print(f"Local DNS: {pattern} -> {gateway_ip}", flush=True)
    try:
        runpy.run_path(str(Path(__file__).with_name(f"{args.example}.py")), run_name="__main__")
    finally:
        socket.getaddrinfo = original


if __name__ == "__main__":
    main()
