"""Run an example with an optional process-local Gateway DNS override."""

from __future__ import annotations

import os
import runpy
import socket
import sys
from ipaddress import ip_address
from pathlib import Path
from typing import Any


def main() -> None:
    original = socket.getaddrinfo
    gateway_ip = os.getenv("DEVBOX_GATEWAY_IP", "").strip()
    if gateway_ip:
        gateway_ip = str(ip_address(gateway_ip))

        def resolve(
            host: bytes | str | None,
            port: bytes | str | int | None,
            *options: Any,
            **kwargs: Any,
        ) -> Any:
            name = host.decode("ascii") if isinstance(host, bytes) else host
            label, separator, _ = (name or "").lower().partition(".")
            if separator and label.endswith("-49983"):
                host = gateway_ip
            return original(host, port, *options, **kwargs)

        # Only DNS changes; the request URL, HTTP Host and TLS SNI stay intact.
        socket.getaddrinfo = resolve
        print(f"Local DNS: *-49983.* -> {gateway_ip}", flush=True)
    original_argv = sys.argv
    try:
        script = str(Path(__file__).with_name("validate_full.py"))
        sys.argv = [script, *original_argv[1:]]
        runpy.run_path(script, run_name="__main__")
    finally:
        sys.argv = original_argv
        socket.getaddrinfo = original


if __name__ == "__main__":
    main()
