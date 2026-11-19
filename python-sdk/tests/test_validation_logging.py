from __future__ import annotations

import runpy
import sys
from pathlib import Path
from unittest.mock import Mock

import pytest

from devbox import CommandExitError, CommandResult, ServiceUnavailableError

example = runpy.run_path(str(Path(__file__).parents[1] / "examples" / "validate_full.py"))


def test_failure_shows_cause(capsys: pytest.CaptureFixture[str]) -> None:
    def operation() -> None:
        try:
            raise OSError("certificate verify failed")
        except OSError as error:
            raise ServiceUnavailableError("unable to reach DevBox service") from error

    validator = example["Validator"]()
    validator.verify("runtime.ready", operation)
    output = capsys.readouterr().out
    assert "FAIL runtime.ready" in output
    assert "OSError: certificate verify failed" in output
    assert "Traceback" in output
    assert validator.failures == ["runtime.ready"]


def test_step_prints_command_output_and_preserves_result(
    capsys: pytest.CaptureFixture[str],
) -> None:
    result = CommandResult(pid=42, exit_code=0, stdout="hello\nworld\n", stderr="warning\n")
    assert example["_step"]("commands.run('hello')", lambda: result) is result
    output = capsys.readouterr().out
    assert "DO commands.run('hello')" in output
    assert "pid=42 | exit_code=0" in output
    assert "stdout:\n      hello\n      world" in output
    assert "stderr:\n      warning" in output


def test_failed_command_prints_stderr_and_still_fails(capsys: pytest.CaptureFixture[str]) -> None:
    def operation() -> None:
        raise CommandExitError(CommandResult(exit_code=127, stderr="git: command not found"))

    validator = example["Validator"]()
    validator.verify("git.workflow", lambda: example["_step"]("git init", operation))
    validator.finish()
    output = capsys.readouterr().out
    assert "git: command not found" in output
    assert "FAIL git.workflow" in output
    assert "SUMMARY tests=1 passed=0 failed=1" in output


def test_step_does_not_dump_connection_objects(capsys: pytest.CaptureFixture[str]) -> None:
    response = {"connectToken": "secret-token", "envdAccessToken": "private-token"}
    assert example["_step"]("connect", lambda: response) is response
    output = capsys.readouterr().out
    assert "secret-token" not in output
    assert "private-token" not in output


def test_command_output_is_bounded(capsys: pytest.CaptureFixture[str]) -> None:
    example["_command_output"](CommandResult(exit_code=0, stdout="x" * 5000))
    output = capsys.readouterr().out
    assert "truncated (5000 characters total)" in output
    assert "(empty)" in output
    assert "x" * 4001 not in output


def test_terminal_validation_rejects_input_echo_without_execution() -> None:
    echoed = "printf '__PTY_42__'; stty size\r\n"
    with pytest.raises(AssertionError):
        example["_validate_terminal_result"](CommandResult(exit_code=7, stdout=echoed))
    example["_validate_terminal_result"](
        CommandResult(exit_code=7, stdout="prompt\r\n__PTY_42__\r\n30 100\r\n")
    )


def test_terminal_output_is_readable_without_changing_raw_result(
    capsys: pytest.CaptureFixture[str],
) -> None:
    raw = "\x1b]0;root@devbox:~\x07\x1b[32mhello\x1b[0m\r\n"
    result = CommandResult(exit_code=0, stdout=raw)
    example["_command_output"](result)
    output = capsys.readouterr().out
    assert "hello" in output
    assert "\x1b" not in output
    assert "root@devbox" not in output
    assert result.stdout == raw


def test_binary_failure_does_not_dump_the_payload() -> None:
    with pytest.raises(AssertionError, match="binary content mismatch") as error:
        example["_equal"](b"a" * 1024, b"b" * 1024)
    assert len(str(error.value)) < 120


@pytest.mark.parametrize(
    "failed", ["sandbox.create", "sandbox.delete", "lifecycle.timeout_kill", "commands.stdout", ""]
)
def test_batch_failure_and_cleanup_policy(monkeypatch: pytest.MonkeyPatch, failed: str) -> None:
    namespace = example["main"].__globals__
    validator = example["Validator"]()
    validator.timings = {"sandbox.create": 0.1, "sandbox.delete": 0.1}
    validator.failures = [failed] if failed else []
    run = Mock(return_value=validator)
    monkeypatch.setitem(namespace, "run_round", run)
    monkeypatch.setattr(sys, "argv", ["validate_full.py", "--mode", "basic", "--rounds", "2"])
    if failed:
        with pytest.raises(SystemExit, match="1"):
            example["main"]()
    else:
        example["main"]()
    expected = 1 if failed in ("sandbox.create", "sandbox.delete", "lifecycle.timeout_kill") else 2
    assert run.call_count == expected
    run.assert_called_with("basic")


def test_local_runner_restores_arguments_and_dns(monkeypatch: pytest.MonkeyPatch) -> None:
    import socket

    runner = runpy.run_path(str(Path(__file__).parents[1] / "examples" / "run_local.py"))
    arguments = ["run_local.py", "--mode", "lifecycle", "--rounds", "2"]
    monkeypatch.setattr(sys, "argv", arguments)
    monkeypatch.setenv("DEVBOX_GATEWAY_IP", "127.0.0.1")
    original = socket.getaddrinfo

    def execute(path: str, *, run_name: str) -> dict[str, object]:
        assert Path(path).name == "validate_full.py"
        assert sys.argv == [path, *arguments[1:]]
        assert run_name == "__main__"
        assert socket.getaddrinfo is not original
        raise RuntimeError("script failed")

    monkeypatch.setattr(runpy, "run_path", execute)
    with pytest.raises(RuntimeError, match="script failed"):
        runner["main"]()
    assert sys.argv is arguments
    assert socket.getaddrinfo is original
