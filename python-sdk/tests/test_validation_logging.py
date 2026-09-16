from __future__ import annotations

import runpy
from pathlib import Path

import pytest

from devbox import CommandExitError, CommandResult

example = runpy.run_path(str(Path(__file__).parents[1] / "examples" / "validate_full.py"))


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
    with pytest.raises(SystemExit) as error:
        validator.finish()
    assert error.value.code == 1
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
