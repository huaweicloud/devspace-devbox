from __future__ import annotations

import runpy
import sys
from pathlib import Path
from types import SimpleNamespace
from unittest.mock import MagicMock, Mock

import httpx
import pytest

from devbox import (
    CommandResult,
    DevBox,
    NotFoundError,
    RequestTimeoutError,
    SandboxState,
    ServiceUnavailableError,
)
from devbox._transport import SyncTransport

example = runpy.run_path(str(Path(__file__).parents[1] / "examples" / "stability.py"))


def client() -> MagicMock:
    api = MagicMock(spec=DevBox)
    api.sandboxes = MagicMock()
    sandbox = api.sandboxes.create.return_value
    sandbox.sandbox_id = "sbx-1"
    sandbox._connection.tunnel_id = "tunnel-1"
    sandbox.commands.run.return_value = CommandResult(exit_code=0, stdout="sbx-1|42")
    sandbox.files.read_bytes.return_value = bytes(range(256)) * 16
    sandbox.kill.return_value = True
    api.sandboxes.connect.return_value = sandbox
    return api


def run(api: MagicMock) -> dict[str, object]:
    row = example["Round"](1)
    example["run_round"](
        api, row, run_id="test", template="default", lifetime=300, checks=1, request_timeout=5
    )
    return {"stop_reason": row.stop_reason, **example["summarize"]([row])}


def test_complete_round_verifies_reconnect_and_cleans_up() -> None:
    api = client()
    summary = run(api)
    assert summary["completed"] == 1
    assert summary["use_success_rate"] == 1
    api.sandboxes.connect.assert_called_once_with("sbx-1")
    api.sandboxes.create.return_value.kill.assert_called_once()


def test_wrong_stdout_is_a_usage_failure_not_a_create_failure() -> None:
    api = client()
    sandbox = api.sandboxes.create.return_value
    sandbox.commands.run.return_value = CommandResult(exit_code=0, stdout="wrong")
    summary = run(api)
    assert summary["create_success"] == 1
    assert summary["create_failed"] == 0
    assert summary["use_failed"] == 1
    assert summary["delete_success"] == 1
    assert summary["completed"] == 0
    sandbox.kill.assert_called_once()


@pytest.mark.parametrize("unknown", [False, True])
def test_create_failure_and_unknown_result_are_distinguished(unknown: bool) -> None:
    api = client()
    api.sandboxes.create.side_effect = (
        RequestTimeoutError("request timed out")
        if unknown
        else ServiceUnavailableError("create failed", code="create_failed", status_code=503)
    )
    summary = run(api)
    assert summary["create_failed"] == 1
    assert summary["use_failed"] == 0
    assert summary["use_success_rate"] is None
    assert bool(summary["stop_reason"]) == unknown


def test_cleanup_failure_stops_further_creation() -> None:
    api = client()
    api.sandboxes.create.return_value.kill.side_effect = ServiceUnavailableError("delete failed")
    summary = run(api)
    assert summary["use_success"] == 1
    assert summary["delete_failed"] == 1
    assert summary["completed"] == 0
    assert summary["stop_reason"]


def test_latency_handles_small_samples() -> None:
    assert example["latency"]([]) == {}
    assert example["latency"]([3])["p95"] == 3
    assert example["latency"](list(range(1, 21)))["p95"] == 19


def test_local_runner_forwards_arguments_and_restores_them(monkeypatch: pytest.MonkeyPatch) -> None:
    runner = runpy.run_path(str(Path(__file__).parents[1] / "examples" / "run_local.py"))
    arguments = ["run_local.py", "stability", "--rounds", "2"]
    monkeypatch.setattr(sys, "argv", arguments)
    monkeypatch.delenv("DEVBOX_GATEWAY_IP", raising=False)

    def execute(path: str, *, run_name: str) -> dict[str, object]:
        assert Path(path).name == "stability.py"
        assert sys.argv == [path, "--rounds", "2"]
        assert run_name == "__main__"
        raise RuntimeError("script failed")

    monkeypatch.setattr(runpy, "run_path", execute)
    with pytest.raises(RuntimeError, match="script failed"):
        runner["main"]()
    assert sys.argv is arguments


def test_http_failure_records_response_without_token(capsys: pytest.CaptureFixture[str]) -> None:
    row = example["Round"](1)
    transport = SyncTransport(
        "https://runtime.test",
        headers={"Cookie": "relay_token=secret-jwt"},
        timeout=1,
        transport=httpx.MockTransport(
            lambda _: httpx.Response(
                404, text="tunnel not found secret-jwt", headers={"X-Request-ID": "server-id"}
            )
        ),
    )
    sandbox = SimpleNamespace(
        _gateway_transport=lambda: transport,
        _connection=SimpleNamespace(connect_token="secret-jwt"),
    )
    try:
        example["capture_http_failures"](row, sandbox)
        with pytest.raises(NotFoundError):
            transport.request("GET", "/files?hidden=value")
        detail = row.http_failures[0]
        assert detail["body"] == "tunnel not found [REDACTED]"
        assert detail["url"] == "https://runtime.test/files"
        assert detail["request_id"] == "server-id"
        assert detail["client_request_id"].startswith("sdk-stability-")
        assert "secret-jwt" not in capsys.readouterr().out
    finally:
        transport.close()


def test_recovery_probe_does_not_turn_failed_round_into_success(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    monkeypatch.setattr("time.sleep", lambda _: None)
    row = example["Round"](1)
    sandbox = Mock()
    sandbox.sandbox_id = "sb1"
    sandbox._connection.tunnel_id = "t1"
    sandbox._gateway_transport.return_value._client.event_hooks = {"request": [], "response": []}
    sandbox.commands.run.side_effect = [
        NotFoundError("missing", status_code=404),
        CommandResult(stdout="sb1|42", exit_code=0),
    ]
    sandbox.get_info.return_value = SimpleNamespace(state=SandboxState.RUNNING, end_at=None)
    sandbox.kill.return_value = True
    client = Mock()
    client.sandboxes.create.return_value = sandbox
    example["run_round"](
        client, row, run_id="r1", template="default", lifetime=300, checks=1, request_timeout=1
    )
    assert not row.use_ok
    assert not row.phases["command.1"].ok
    assert row.phases["diagnostic.command"].ok
    assert example["summarize"]([row])["use_failed"] == 1
    sandbox.kill.assert_called_once()
