"""Serial create/use/delete stability sampling against a real deployment."""

from __future__ import annotations

import argparse
import json
import math
import os
import statistics
import tempfile
import time
from collections.abc import Callable
from dataclasses import asdict, dataclass, field
from datetime import datetime, timezone
from pathlib import Path
from typing import TypeVar, cast
from uuid import uuid4

from devbox import DevBox, DevBoxError, Sandbox
from devbox.config import ConnectionConfig

T = TypeVar("T")


@dataclass
class Phase:
    seconds: float
    ok: bool
    error: str = ""


@dataclass
class Round:
    number: int
    started_at: str = field(default_factory=lambda: datetime.now(timezone.utc).isoformat())
    sandbox_id: str = ""
    tunnel_id: str = ""
    phases: dict[str, Phase] = field(default_factory=dict)
    use_ok: bool = False
    stop_reason: str = ""


def measure(row: Round, name: str, action: Callable[[], T]) -> T:
    started = time.monotonic()
    try:
        value = action()
    except (Exception, KeyboardInterrupt) as error:
        message = f"{type(error).__name__}: {' '.join(str(error).split())}"[:500]
        if isinstance(error, DevBoxError):
            message += f" | http={error.status_code} request_id={error.request_id}"
        row.phases[name] = Phase(time.monotonic() - started, False, message)
        print(f"  FAIL {name:<12} {row.phases[name].seconds:7.3f}s | {message}", flush=True)
        raise
    row.phases[name] = Phase(time.monotonic() - started, True)
    detail = f" | {value}" if isinstance(value, (str, bool)) else ""
    print(f"  PASS {name:<12} {row.phases[name].seconds:7.3f}s{detail}", flush=True)
    return value


def command_probe(sandbox: Sandbox, timeout: float) -> str:
    result = sandbox.commands.run('printf "%s|%s" "$DEVBOX_ID" "$((19+23))"', timeout=timeout)
    expected = f"{sandbox.sandbox_id}|42"
    if result.stdout != expected or result.stderr:
        raise AssertionError(
            f"expected {expected!r}; stdout={result.stdout!r}, stderr={result.stderr!r}"
        )
    return f"stdout={result.stdout!r} exit={result.exit_code}"


def file_probe(sandbox: Sandbox, path: str, content: bytes) -> str:
    sandbox.files.write(path, content)
    if sandbox.files.read_bytes(path) != content:
        raise AssertionError("file content changed during transfer")
    return f"bytes={len(content)} exact_match=true"


def reconnect_probe(client: DevBox, row: Round, path: str, content: bytes, timeout: float) -> str:
    attached = client.sandboxes.connect(row.sandbox_id)
    try:
        command_probe(attached, timeout)
        if attached.files.read_bytes(path) != content:
            raise AssertionError("file content changed after reconnect")
        return "identity=matched file=preserved"
    finally:
        attached.close()


def run_round(
    client: DevBox,
    row: Round,
    *,
    run_id: str,
    template: str,
    lifetime: int,
    checks: int,
    request_timeout: float,
) -> None:
    sandbox: Sandbox | None = None
    try:
        sandbox = measure(
            row,
            "create",
            lambda: client.sandboxes.create(
                template,
                timeout=lifetime,
                metadata={"sdk_stability": run_id, "round": str(row.number)},
            ),
        )
        row.sandbox_id = sandbox.sandbox_id
        row.tunnel_id = sandbox._connection.tunnel_id
        print(f"  sandbox_id={row.sandbox_id} tunnel_id={row.tunnel_id}", flush=True)
        for index in range(checks):
            measure(row, f"command.{index + 1}", lambda: command_probe(sandbox, request_timeout))
            if index + 1 < checks:
                time.sleep(1)
        path = f"/tmp/sdk-stability-{run_id}.bin"
        content = bytes(range(256)) * 16
        measure(row, "files", lambda: file_probe(sandbox, path, content))
        sandbox.close()
        measure(
            row, "reconnect", lambda: reconnect_probe(client, row, path, content, request_timeout)
        )
        row.use_ok = True
    except Exception as error:
        if sandbox is None and (not isinstance(error, DevBoxError) or error.status_code is None):
            row.stop_reason = "create outcome unknown; inspect this run's metadata before retrying"
    finally:
        if sandbox is not None:
            try:
                measure(row, "delete", sandbox.kill)
            except Exception:
                row.stop_reason = (
                    f"cleanup failed; inspect sandbox {row.sandbox_id} before continuing"
                )
            finally:
                sandbox.close()


def latency(values: list[float]) -> dict[str, float]:
    ordered = sorted(values)
    if not ordered:
        return {}
    return {
        "min": ordered[0],
        "avg": statistics.mean(ordered),
        "p50": statistics.median(ordered),
        "p95": ordered[math.ceil(len(ordered) * 0.95) - 1],
        "max": ordered[-1],
    }


def summarize(rows: list[Round]) -> dict[str, object]:
    created = [row for row in rows if row.sandbox_id]
    used = sum(row.use_ok for row in created)
    deleted = sum(row.phases.get("delete", Phase(0, False)).ok for row in created)
    completed = sum(row.use_ok and row.phases.get("delete", Phase(0, False)).ok for row in rows)
    return {
        "attempted": len(rows),
        "create_success": len(created),
        "create_failed": len(rows) - len(created),
        "use_success": used,
        "use_failed": len(created) - used,
        "delete_success": deleted,
        "delete_failed": len(created) - deleted,
        "completed": completed,
        "create_success_rate": len(created) / len(rows) if rows else None,
        "use_success_rate": used / len(created) if created else None,
        "create_success_seconds": latency([row.phases["create"].seconds for row in created]),
        "create_failed_seconds": latency(
            [
                row.phases["create"].seconds
                for row in rows
                if "create" in row.phases and not row.phases["create"].ok
            ]
        ),
    }


def print_summary(rows: list[Round], summary: dict[str, object]) -> None:
    print(f"\nSUMMARY attempted={summary['attempted']} complete={summary['completed']}")
    for stage in ("create", "use", "delete"):
        success = cast(int, summary[f"{stage}_success"])
        failed = cast(int, summary[f"{stage}_failed"])
        rate = f"{success / (success + failed):.1%}" if success + failed else "n/a"
        print(f"  {stage:<8} success={success} failed={failed} success_rate={rate}")
    print("  LATENCY seconds (successful phases; create failures shown separately)")
    for name in ("create", "create_failed", "command", "files", "reconnect", "delete"):
        group = "create" if name == "create_failed" else name
        values = [
            phase.seconds
            for row in rows
            for key, phase in row.phases.items()
            if key.split(".")[0] == group and phase.ok == (name != "create_failed")
        ]
        stats = latency(values)
        line = " ".join(f"{key}={value:.3f}" for key, value in stats.items()) or "n/a"
        print(f"  {name:<14} n={len(values):<4} {line}")


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--rounds", type=int, default=20)
    parser.add_argument(
        "--checks", type=int, default=3, help="command probes per sandbox, 1s apart"
    )
    parser.add_argument("--interval", type=float, default=2, help="seconds between rounds")
    parser.add_argument("--lifetime", type=int, default=300, help="sandbox lifetime in seconds")
    parser.add_argument("--request-timeout", type=float, default=30)
    parser.add_argument("--template", default=os.getenv("DEVBOX_TEST_TEMPLATE") or "default")
    parser.add_argument(
        "--report", type=Path, help="JSON report path (default: temporary directory)"
    )
    args = parser.parse_args()
    if min(args.rounds, args.checks, args.lifetime, args.request_timeout) <= 0 or args.interval < 0:
        parser.error("counts and timeouts must be positive; interval must not be negative")
    config = ConnectionConfig.resolve(request_timeout=args.request_timeout)
    if config.gateway_url and "{tunnel_id}" not in config.gateway_url:
        parser.error("stability testing requires a dynamic gateway URL, not a fixed sandbox")
    run_id = uuid4().hex[:12]
    report = args.report or Path(tempfile.gettempdir()) / f"devbox-stability-{run_id}.json"
    report.parent.mkdir(parents=True, exist_ok=True)
    rows: list[Round] = []
    started_at = datetime.now(timezone.utc).isoformat()
    started = time.monotonic()
    interrupted = False
    print(f"STABILITY run={run_id} started={started_at}")
    print(f"manager={config.api_url} gateway={config.gateway_url or 'manager-provided'}")
    print(
        f"rounds={args.rounds} template={args.template} lifetime={args.lifetime}s "
        f"checks={args.checks} interval={args.interval}s request_timeout={args.request_timeout}s"
    )
    print(
        "mode=serial | no application retries | "
        "failed cleanup or unknown create outcome stops the run"
    )
    print(f"report={report.resolve()}", flush=True)
    try:
        with DevBox(request_timeout=args.request_timeout) as client:
            for number in range(1, args.rounds + 1):
                row = Round(number)
                rows.append(row)
                timestamp = datetime.now().astimezone().isoformat(timespec="seconds")
                print(
                    f"\nROUND {number:03d}/{args.rounds} | {timestamp}",
                    flush=True,
                )
                run_round(
                    client,
                    row,
                    run_id=run_id,
                    template=args.template,
                    lifetime=args.lifetime,
                    checks=args.checks,
                    request_timeout=args.request_timeout,
                )
                if row.stop_reason:
                    print(f"STOP {row.stop_reason}", flush=True)
                    break
                if number < args.rounds:
                    time.sleep(args.interval)
    except KeyboardInterrupt:
        interrupted = True
        print("\nSTOP interrupted; summarizing completed attempts", flush=True)
    finally:
        summary = summarize(rows)
        data = {
            "run_id": run_id,
            "started_at": started_at,
            "elapsed_seconds": time.monotonic() - started,
            "requested_rounds": args.rounds,
            "settings": {
                "template": args.template,
                "lifetime_seconds": args.lifetime,
                "checks": args.checks,
                "interval_seconds": args.interval,
                "request_timeout_seconds": args.request_timeout,
                "manager_url": config.api_url,
                "gateway_url": config.gateway_url,
            },
            "interrupted": interrupted,
            "summary": summary,
            "rounds": [asdict(row) for row in rows],
        }
        report.write_text(json.dumps(data, indent=2) + "\n", encoding="utf-8")
        print_summary(rows, summary)
        print(f"elapsed={data['elapsed_seconds']:.1f}s report={report.resolve()}", flush=True)
    if interrupted:
        raise SystemExit(130)
    if summary["completed"] != args.rounds:
        raise SystemExit(1)


if __name__ == "__main__":
    main()
