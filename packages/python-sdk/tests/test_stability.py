from __future__ import annotations

import runpy
import sys
from pathlib import Path
from unittest.mock import MagicMock

import pytest

from devbox import CommandResult, DevBox, RequestTimeoutError, ServiceUnavailableError

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
