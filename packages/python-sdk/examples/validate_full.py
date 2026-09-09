from __future__ import annotations

import os
import tempfile
import time
from collections.abc import Callable
from pathlib import Path
from typing import TypeVar
from uuid import uuid4

from devbox import CommandResult, DevBox, PtySize, Sandbox, ServiceUnavailableError

T = TypeVar("T")


class Validator:
    def __init__(self) -> None:
        self.failures: list[str] = []
        self.tests = 0

    def verify(self, name: str, operation: Callable[[], T]) -> T | None:
        self.tests += 1
        started_at = time.monotonic()
        try:
            result = operation()
        except Exception as error:
            self.failures.append(name)
            print(
                f"FAIL {name} | {_seconds(started_at)}s | {type(error).__name__}: {error}",
                flush=True,
            )
            return None
        print(f"PASS {name} | {_seconds(started_at)}s", flush=True)
        return result


def main() -> None:
    template = os.getenv("DEVBOX_TEST_TEMPLATE", "default").strip() or "default"
    validator = Validator()
    client = DevBox()
    sandbox: Sandbox | None = None

    print(f"DevBox full validation | template={template}")
    try:
        sandbox = validator.verify(
            "sandbox.create",
            lambda: client.sandboxes.create(
                template,
                timeout=300,
                metadata={"sdk_validation": "python"},
            ),
        )
        if sandbox is None:
            print("STOP sandbox creation failed; runtime validation cannot continue")
        else:
            print(f"sandbox_id={sandbox.sandbox_id}")
            validate_sandbox(client, sandbox, validator)
    finally:
        if sandbox is not None:
            validator.verify("sandbox.delete", sandbox.kill)
            sandbox.close()
        client.close()

    passed = validator.tests - len(validator.failures)
    print(f"SUMMARY tests={validator.tests} passed={passed} failed={len(validator.failures)}")
    if validator.failures:
        print(f"FAILED {', '.join(validator.failures)}")
        raise SystemExit(1)


def validate_sandbox(client: DevBox, sandbox: Sandbox, validator: Validator) -> None:
    validator.verify(
        "manager.get",
        lambda: _equal(sandbox.get_info().sandbox_id, sandbox.sandbox_id),
    )
    validator.verify("manager.is_running", lambda: _equal(sandbox.is_running(), True))
    validator.verify("manager.list", lambda: _validate_list(client, sandbox))
    validator.verify("manager.set_timeout", lambda: sandbox.set_timeout(300))
    validator.verify("manager.refresh", lambda: sandbox.refresh(300))
    validator.verify("manager.metrics", sandbox.get_metrics)
    validator.verify("manager.logs", lambda: sandbox.get_logs(limit=20))
    runtime_ready = validator.verify("runtime.ready", lambda: _validate_runtime(sandbox))
    if runtime_ready is None:
        print("SKIP commands, filesystem, PTY and Git: runtime gateway is unavailable")
        return

    validate_commands(sandbox, validator)
    validate_filesystem(sandbox, validator)
    validate_pty(sandbox, validator)
    validate_git(sandbox, validator)


def validate_commands(sandbox: Sandbox, validator: Validator) -> None:
    validator.verify("commands.foreground", lambda: _foreground_command(sandbox))
    validator.verify("commands.background", lambda: _background_command(sandbox))
    validator.verify("commands.stdin", lambda: _stdin_command(sandbox))


def validate_filesystem(sandbox: Sandbox, validator: Validator) -> None:
    root = f"/tmp/devbox-sdk-{uuid4().hex[:8]}"
    validator.verify("filesystem.basic", lambda: _filesystem_basic(sandbox, root))
    validator.verify("filesystem.transfer", lambda: _filesystem_transfer(sandbox, root))
    validator.verify("filesystem.remove", lambda: sandbox.files.remove(root))


def validate_pty(sandbox: Sandbox, validator: Validator) -> None:
    def interaction() -> None:
        session = sandbox.pty.start(size=PtySize(rows=24, cols=80))
        sandbox.pty.resize(session.pid, PtySize(rows=30, cols=100))
        session.send_stdin("printf pty-ok\\nexit\\n")
        result = session.wait(check=False)
        _equal(result.exit_code, 0)
        _contains(result.stdout, "pty-ok")

    validator.verify("pty.interaction", interaction)


def validate_git(sandbox: Sandbox, validator: Validator) -> None:
    repository = f"/tmp/devbox-sdk-git-{uuid4().hex[:8]}"

    def workflow() -> None:
        sandbox.commands.run(f"mkdir -p {repository} && git -C {repository} init")
        sandbox.git.set_config(repository, "user.name", "DevBox SDK")
        sandbox.git.set_config(repository, "user.email", "sdk@devbox.local")
        sandbox.files.write(f"{repository}/README.md", "test\n")
        sandbox.git.add(repository)
        sandbox.git.commit(repository, "test commit")
        _equal("README.md" in sandbox.git.status(repository).stdout, False)

    validator.verify("git.workflow", workflow)
    validator.verify("git.remove", lambda: sandbox.files.remove(repository))


def _validate_list(client: DevBox, sandbox: Sandbox) -> None:
    page = client.sandboxes.list(limit=100)
    _equal(any(item.sandbox_id == sandbox.sandbox_id for item in page.items), True)


def _validate_runtime(sandbox: Sandbox) -> bool:
    deadline = time.monotonic() + 30
    while True:
        try:
            result = sandbox.commands.run("printf runtime-ready")
            _expect_result(result, stdout="runtime-ready")
            return True
        except ServiceUnavailableError:
            if time.monotonic() >= deadline:
                raise
            time.sleep(1)


def _foreground_command(sandbox: Sandbox) -> None:
    result = sandbox.commands.run(
        'printf "$DEVBOX_TEST"; printf error-ok >&2',
        envs={"DEVBOX_TEST": "command-ok"},
        cwd="/tmp",
    )
    _expect_result(result, stdout="command-ok", stderr="error-ok")


def _background_command(sandbox: Sandbox) -> None:
    process = sandbox.commands.run(
        "sleep 1; printf background-ok",
        background=True,
        timeout=10,
    )
    _expect_result(process.wait(), stdout="background-ok")


def _stdin_command(sandbox: Sandbox) -> None:
    process = sandbox.commands.run("cat", background=True, stdin=True, timeout=10)
    process.send_stdin("stdin-ok\n")
    process.close_stdin()
    _expect_result(process.wait(), stdout="stdin-ok\n")


def _filesystem_basic(sandbox: Sandbox, root: str) -> None:
    sandbox.files.make_dir(root)
    path = f"{root}/input.txt"
    moved = f"{root}/output.txt"
    sandbox.files.write(path, "file-ok")
    _equal(sandbox.files.read(path), "file-ok")
    _equal(sandbox.files.stat(path).size, 7)
    _equal(sandbox.files.exists(path), True)
    _equal(any(item.name == "input.txt" for item in sandbox.files.list(root)), True)
    sandbox.files.move(path, moved)
    _equal(sandbox.files.exists(moved), True)


def _filesystem_transfer(sandbox: Sandbox, root: str) -> None:
    with tempfile.TemporaryDirectory() as directory:
        source = Path(directory, "source.txt")
        target = Path(directory, "target.txt")
        source.write_text("transfer-ok", encoding="utf-8")
        sandbox.files.upload(source, f"{root}/transfer.txt")
        sandbox.files.download(f"{root}/transfer.txt", target)
        _equal(target.read_text(encoding="utf-8"), "transfer-ok")


def _expect_result(
    result: CommandResult,
    *,
    stdout: str | None = None,
    stderr: str | None = None,
) -> None:
    _equal(result.exit_code, 0)
    if stdout is not None:
        _equal(result.stdout, stdout)
    if stderr is not None:
        _equal(result.stderr, stderr)


def _equal(actual: object, expected: object) -> None:
    if actual != expected:
        raise AssertionError(f"expected {expected!r}, received {actual!r}")


def _contains(value: str, expected: str) -> None:
    if expected not in value:
        raise AssertionError(f"expected {expected!r} in {value!r}")


def _seconds(started_at: float) -> str:
    return f"{time.monotonic() - started_at:.3f}"


if __name__ == "__main__":
    main()
