from __future__ import annotations

import os
import tempfile
import time
from collections.abc import Callable
from pathlib import Path
from typing import TypeVar
from uuid import uuid4

import httpx

from devbox import (
    CommandExitError,
    CommandHandle,
    CommandResult,
    DevBox,
    NotFoundError,
    PtySize,
    Sandbox,
    SandboxInfo,
    SandboxMetrics,
    ServiceUnavailableError,
)
from devbox._tls import service_ssl_context
from devbox.config import ConnectionConfig

T = TypeVar("T")


class Validator:
    def __init__(self) -> None:
        self.failures: list[str] = []
        self.tests = 0

    def verify(self, name: str, operation: Callable[[], T]) -> T | None:
        self.tests += 1
        started_at = time.monotonic()
        print(f"\nTEST {self.tests:02d} | {name}", flush=True)
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

    def finish(self) -> None:
        passed = self.tests - len(self.failures)
        print(f"SUMMARY tests={self.tests} passed={passed} failed={len(self.failures)}")
        if self.failures:
            print(f"FAILED {', '.join(self.failures)}")
            raise SystemExit(1)


def main() -> None:
    template = os.getenv("DEVBOX_TEST_TEMPLATE", "default").strip() or "default"
    validator = Validator()
    sandbox: Sandbox | None = None

    print(f"DevBox full validation | template={template}")
    gateway_url = ConnectionConfig.resolve().gateway_url
    if gateway_url:
        print(f"gateway={gateway_url}")
    if gateway_url and "{tunnel_id}" not in gateway_url:
        print("MODE fixed gateway: Manager and runtime are tested independently")
        if validator.verify("runtime.endpoint", lambda: _check_endpoint(gateway_url)) is None:
            print("STOP fixed runtime endpoint is unavailable; no sandbox was created")
            validator.finish()
            return

    client = DevBox()
    try:
        _section(
            "Create sandbox", f"Start template={template!r}, timeout=300s, with an isolated runtime"
        )
        sandbox = validator.verify(
            "sandbox.create",
            lambda: _step(
                "sandboxes.create(envs={'DEVBOX_SDK_ENV': 'created-731'})",
                lambda: client.sandboxes.create(
                    template,
                    timeout=300,
                    envs={"DEVBOX_SDK_ENV": "created-731"},
                    metadata={"sdk_validation": "python"},
                ),
            ),
        )
        if sandbox is None:
            print("STOP sandbox creation failed; runtime validation cannot continue")
        else:
            print(f"sandbox_id={sandbox.sandbox_id}")
            validate_sandbox(client, sandbox, validator)
    finally:
        if sandbox is not None:
            _section("Cleanup", "Delete the remote sandbox and close local connections")
            validator.verify("sandbox.delete", lambda: _step("sandbox.kill()", sandbox.kill))
            sandbox.close()
        client.close()

    validator.finish()


def _check_endpoint(gateway_url: str) -> bool:
    url = gateway_url.replace("{port}", "49983").rstrip("/") + "/health"
    with httpx.Client(verify=service_ssl_context(), timeout=10, follow_redirects=False) as client:
        response = client.get(url)
    if not response.is_success:
        reason = ""
        if response.headers.get("content-type", "").startswith("text/plain"):
            reason = " | " + " ".join(response.text.split())[:200]
        raise RuntimeError(f"GET {url} | HTTP {response.status_code}{reason}")
    return True


def _validate_connect(client: DevBox, sandbox: Sandbox) -> None:
    connected = _step(
        f"sandboxes.connect({sandbox.sandbox_id!r}) | open a new SDK handle",
        lambda: client.sandboxes.connect(sandbox.sandbox_id),
    )
    try:
        _equal(connected.sandbox_id, sandbox.sandbox_id)
    finally:
        _step("connected.close() | keep the remote sandbox running", connected.close)


def validate_sandbox(client: DevBox, sandbox: Sandbox, validator: Validator) -> None:
    _section("Sandbox management", "Inspect, reconnect, extend lifetime and read monitoring data")
    validator.verify(
        "manager.get",
        lambda: _equal(
            _step("sandbox.get_info()", sandbox.get_info).sandbox_id, sandbox.sandbox_id
        ),
    )
    validator.verify(
        "manager.is_running",
        lambda: _equal(_step("sandbox.is_running()", sandbox.is_running), True),
    )
    validator.verify("manager.list", lambda: _validate_list(client, sandbox))
    validator.verify("manager.connect", lambda: _validate_connect(client, sandbox))
    validator.verify(
        "manager.set_timeout",
        lambda: _step(
            "sandbox.set_timeout(300) | remaining lifetime: 300s", lambda: sandbox.set_timeout(300)
        ),
    )
    validator.verify(
        "manager.refresh",
        lambda: _validate_refresh(sandbox),
    )
    validator.verify("manager.metrics", lambda: _step("sandbox.get_metrics()", sandbox.get_metrics))
    validator.verify(
        "manager.logs",
        lambda: _step("sandbox.get_logs(limit=20)", lambda: sandbox.get_logs(limit=20)),
    )
    runtime_ready = validator.verify("runtime.ready", lambda: _validate_runtime(sandbox))
    if runtime_ready is None:
        print("SKIP commands, filesystem, PTY and Git: runtime gateway is unavailable")
        return

    gateway_url = ConnectionConfig.resolve().gateway_url
    if gateway_url and "{tunnel_id}" not in gateway_url:
        print("SKIP runtime.environment: fixed gateway is independent of the created sandbox")
    else:
        validator.verify("runtime.environment", lambda: _validate_environment(sandbox))

    validate_commands(sandbox, validator)
    validate_filesystem(sandbox, validator)
    validate_pty(sandbox, validator)
    validate_git(sandbox, validator)


def validate_commands(sandbox: Sandbox, validator: Validator) -> None:
    _section("Commands", "Capture stdout/stderr, run background jobs and send standard input")
    validator.verify("commands.foreground", lambda: _foreground_command(sandbox))
    validator.verify("commands.background", lambda: _background_command(sandbox))
    validator.verify("commands.stdin", lambda: _stdin_command(sandbox))
    validator.verify("commands.reconnect", lambda: _reconnect_command(sandbox))


def validate_filesystem(sandbox: Sandbox, validator: Validator) -> None:
    _section("Files", "Create, read, inspect, move, upload, download and remove files")
    root = f"/tmp/devbox-sdk-{uuid4().hex[:8]}"
    validator.verify("filesystem.basic", lambda: _filesystem_basic(sandbox, root))
    validator.verify("filesystem.transfer", lambda: _filesystem_transfer(sandbox, root))
    validator.verify(
        "filesystem.remove",
        lambda: _step(f"files.remove({root!r})", lambda: sandbox.files.remove(root)),
    )


def validate_pty(sandbox: Sandbox, validator: Validator) -> None:
    _section("Interactive terminal", "Start Bash, resize the terminal, send input and exit")

    def interaction() -> None:
        session = _step(
            "pty.start(rows=24, cols=80)", lambda: sandbox.pty.start(size=PtySize(rows=24, cols=80))
        )
        _step(
            f"pty.resize(pid={session.pid}, rows=30, cols=100)",
            lambda: sandbox.pty.resize(session.pid, PtySize(rows=30, cols=100)),
        )
        command = "printf '\\n__PTY_%s__\\n' \"$((19+23))\"; stty size; exit 7\n"
        _step(
            f"session.send_stdin({command!r}) | compute 42, inspect size, exit with code 7",
            lambda: session.send_stdin(command),
        )
        result = _step(
            "session.wait() | read terminal output until Bash exits",
            lambda: session.wait(check=False),
        )
        _validate_terminal_result(result)
        try:
            session.send_stdin("must-not-run\n")
        except RuntimeError:
            print("    completed session rejects further input", flush=True)
        else:
            raise AssertionError("completed terminal accepted input")

    validator.verify("pty.interaction", interaction)


def validate_git(sandbox: Sandbox, validator: Validator) -> None:
    _section("Git", "Initialize a repository, configure an author, stage and commit a file")
    repository = f"/tmp/devbox-sdk-git-{uuid4().hex[:8]}"

    def workflow() -> None:
        command = f"mkdir -p {repository} && git -C {repository} init -b main"
        _step(f"commands.run({command!r})", lambda: sandbox.commands.run(command))
        _step(
            "git.set_config(user.name='DevBox SDK')",
            lambda: sandbox.git.set_config(repository, "user.name", "DevBox SDK"),
        )
        _step(
            "git.set_config(user.email='sdk@devbox.local')",
            lambda: sandbox.git.set_config(repository, "user.email", "sdk@devbox.local"),
        )
        _step(
            "files.write(README.md, 'test\\n')",
            lambda: sandbox.files.write(f"{repository}/README.md", "test\n"),
        )
        _step("git.add() | stage changes", lambda: sandbox.git.add(repository))
        _step("git.commit('test commit')", lambda: sandbox.git.commit(repository, "test commit"))
        committed = _step(
            "git show HEAD:README.md | verify committed content, not just exit code",
            lambda: sandbox.commands.run(f"git -C {repository} show HEAD:README.md"),
        )
        _expect_result(committed, stdout="test\n", stderr="")
        result = _step(
            "git.status() | expect a clean working tree", lambda: sandbox.git.status(repository)
        )
        _equal(result.stdout.strip(), "## main")

    validator.verify("git.workflow", workflow)
    validator.verify(
        "git.remove",
        lambda: _step(f"files.remove({repository!r})", lambda: sandbox.files.remove(repository)),
    )


def _validate_refresh(sandbox: Sandbox) -> None:
    before = time.time()
    _step("sandbox.refresh(300) | reset deadline to now + 300s", lambda: sandbox.refresh(300))
    info = _step("sandbox.get_info() | verify the actual deadline", sandbox.get_info)
    if info.end_at is None or not before + 295 <= info.end_at.timestamp() <= time.time() + 305:
        raise AssertionError(f"refresh did not reset the deadline: {info.end_at}")


def _validate_terminal_result(result: CommandResult) -> None:
    lines = result.stdout.replace("\r", "").splitlines()
    _equal(result.exit_code, 7)
    _equal("__PTY_42__" in lines, True)
    _equal("30 100" in lines, True)


def _validate_environment(sandbox: Sandbox) -> None:
    command = 'printf "%s|%s" "$DEVBOX_ID" "$DEVBOX_SDK_ENV"'
    result = _step(
        f"commands.run({command!r}) | verify sandbox identity and create-time environment",
        lambda: sandbox.commands.run(command),
    )
    _expect_result(result, stdout=f"{sandbox.sandbox_id}|created-731", stderr="")


def _validate_list(client: DevBox, sandbox: Sandbox) -> None:
    page = _step("sandboxes.list(limit=100)", lambda: client.sandboxes.list(limit=100))
    print(f"    sandboxes={len(page.items)} | current sandbox must be present", flush=True)
    _equal(any(item.sandbox_id == sandbox.sandbox_id for item in page.items), True)


def _validate_runtime(sandbox: Sandbox) -> bool:
    gateway_url = ConnectionConfig.resolve().gateway_url
    if gateway_url:
        gateway_url = gateway_url.replace("{tunnel_id}", sandbox._connection.tunnel_id)
        print(f"runtime_url={gateway_url.replace('{port}', '49983')}", flush=True)
    deadline = time.monotonic() + 30
    while True:
        try:
            result = _step(
                "commands.run('printf runtime-ready') | verify remote execution",
                lambda: sandbox.commands.run("printf runtime-ready"),
            )
            _expect_result(result, stdout="runtime-ready")
            return True
        except NotFoundError:
            if gateway_url:
                _check_endpoint(gateway_url)
            raise
        except ServiceUnavailableError:
            if time.monotonic() >= deadline:
                raise
            print("    RETRY runtime is not ready; wait 1s (maximum 30s)", flush=True)
            time.sleep(1)


def _foreground_command(sandbox: Sandbox) -> None:
    command = 'printf "$DEVBOX_TEST"; printf error-ok >&2'
    result = _step(
        f"commands.run({command!r}) | cwd=/tmp, DEVBOX_TEST=command-ok",
        lambda: sandbox.commands.run(
            command,
            envs={"DEVBOX_TEST": "command-ok"},
            cwd="/tmp",
        ),
    )
    _expect_result(result, stdout="command-ok", stderr="error-ok")

    python_command = 'python3 -c "print(40 + 2)"'
    result = _step(
        f"commands.run({python_command!r}) | execute Python",
        lambda: sandbox.commands.run(
            python_command,
            cwd="/tmp",
        ),
    )
    _expect_result(result, stdout="42\n", stderr="")

    failed = _step(
        "commands.run('printf partial; printf deliberate >&2; exit 7', check=False)",
        lambda: sandbox.commands.run("printf partial; printf deliberate >&2; exit 7", check=False),
    )
    _equal((failed.exit_code, failed.stdout, failed.stderr), (7, "partial", "deliberate"))


def _reconnect_command(sandbox: Sandbox) -> None:
    process = _step(
        "commands.run('read line; printf reconnected:%s \"$line\"', background=True, stdin=True)",
        lambda: sandbox.commands.run(
            'read line; printf reconnected:%s "$line"', background=True, stdin=True, timeout=10
        ),
    )
    _step("process.disconnect() | remote process stays alive", process.disconnect)
    attached = _step(
        f"commands.connect({process.pid})",
        lambda: sandbox.commands.connect(process.pid, timeout=10),
    )
    _step(r"attached.send_stdin('proof-731\n')", lambda: attached.send_stdin("proof-731\n"))
    _expect_result(
        _step("attached.wait() | verify output after reconnect", attached.wait),
        stdout="reconnected:proof-731",
        stderr="",
    )


def _background_command(sandbox: Sandbox) -> None:
    process = _step(
        "commands.run('sleep 1; printf background-ok', background=True, timeout=10)",
        lambda: sandbox.commands.run(
            "sleep 1; printf background-ok",
            background=True,
            timeout=10,
        ),
    )
    print(
        "    SDK returned a handle; the remote process can continue in the background", flush=True
    )
    result = _step(
        f"process.wait(pid={process.pid}) | wait for completion and collect output", process.wait
    )
    _expect_result(result, stdout="background-ok")


def _stdin_command(sandbox: Sandbox) -> None:
    process = _step(
        "commands.run('cat', background=True, stdin=True, timeout=10)",
        lambda: sandbox.commands.run("cat", background=True, stdin=True, timeout=10),
    )
    _step(r"process.send_stdin('stdin-ok\n')", lambda: process.send_stdin("stdin-ok\n"))
    _step("process.close_stdin() | send EOF so cat can finish", process.close_stdin)
    _expect_result(
        _step("process.wait() | collect the echoed input", process.wait), stdout="stdin-ok\n"
    )


def _filesystem_basic(sandbox: Sandbox, root: str) -> None:
    _step(f"files.make_dir({root!r})", lambda: sandbox.files.make_dir(root))
    path = f"{root}/input.txt"
    moved = f"{root}/output.txt"
    _step(f"files.write({path!r}, 'file-ok')", lambda: sandbox.files.write(path, "file-ok"))
    _equal(_step(f"files.read({path!r})", lambda: sandbox.files.read(path)), "file-ok")
    _equal(_step(f"files.stat({path!r}) | size in bytes", lambda: sandbox.files.stat(path).size), 7)
    _equal(_step(f"files.exists({path!r})", lambda: sandbox.files.exists(path)), True)
    entries = _step(f"files.list({root!r})", lambda: sandbox.files.list(root))
    print(f"    entries={[item.name for item in entries]}", flush=True)
    _equal(any(item.name == "input.txt" for item in entries), True)
    _step(f"files.move({path!r}, {moved!r})", lambda: sandbox.files.move(path, moved))
    _equal(_step(f"files.exists({moved!r})", lambda: sandbox.files.exists(moved)), True)


def _filesystem_transfer(sandbox: Sandbox, root: str) -> None:
    binary_path = f"{root}/binary.dat"
    binary = bytes(range(256)) * 4096
    _step(
        f"files.write({binary_path!r}, <1 MiB binary data>)",
        lambda: sandbox.files.write(binary_path, binary),
    )
    downloaded = _step(
        f"files.read_bytes({binary_path!r})", lambda: sandbox.files.read_bytes(binary_path)
    )
    if downloaded != binary:
        raise AssertionError("binary file content changed during transfer")
    print(f"    verified {len(binary)} bytes, including NUL and non-UTF-8 bytes", flush=True)
    with tempfile.TemporaryDirectory() as directory:
        source = Path(directory, "source.txt")
        target = Path(directory, "target.txt")
        source.write_text("transfer-ok", encoding="utf-8")
        _step(
            f"files.upload({str(source)!r}, {root + '/transfer.txt'!r}) | content='transfer-ok'",
            lambda: sandbox.files.upload(source, f"{root}/transfer.txt"),
        )
        _step(
            f"files.download({root + '/transfer.txt'!r}, {str(target)!r})",
            lambda: sandbox.files.download(f"{root}/transfer.txt", target),
        )
        _equal(
            _step("Read downloaded local file", lambda: target.read_text(encoding="utf-8")),
            "transfer-ok",
        )


def _section(name: str, description: str) -> None:
    print(f"\n=== {name} ===\n{description}", flush=True)


def _step(description: str, operation: Callable[[], T]) -> T:
    print(f"  DO {description}", flush=True)
    started = time.monotonic()
    try:
        result = operation()
    except CommandExitError as error:
        _command_output(error.result)
        raise
    print(f"    DONE | {_seconds(started)}s", flush=True)
    if isinstance(result, CommandResult):
        _command_output(result)
    elif isinstance(result, CommandHandle):
        print(f"    pid={result.pid}", flush=True)
    elif isinstance(result, SandboxInfo):
        print(
            f"    state={result.state.value} | template={result.template_id}"
            f" | end_at={result.end_at}",
            flush=True,
        )
    elif isinstance(result, (str, bool, int)):
        print(f"    result={result!r}", flush=True)
    elif isinstance(result, (list, tuple)):
        print(f"    items={len(result)}", flush=True)
        if result and isinstance(result[-1], SandboxMetrics):
            sample = result[-1]
            print(
                f"    sample_time={sample.timestamp_unix} | cpu_used={sample.cpu_used_percent}%"
                f" | memory_used={sample.memory_used_bytes} bytes"
                f" | disk_used={sample.disk_used_bytes} bytes",
                flush=True,
            )
    return result


def _command_output(result: CommandResult) -> None:
    print(f"    pid={result.pid} | exit_code={result.exit_code}", flush=True)
    for name, value in (("stdout", result.stdout), ("stderr", result.stderr)):
        print(f"    {name}:", flush=True)
        for line in value[:4000].splitlines() or ["(empty)"]:
            print(f"      {line}", flush=True)
        if len(value) > 4000:
            print(f"      ... truncated ({len(value)} characters total)", flush=True)


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


def _seconds(started_at: float) -> str:
    return f"{time.monotonic() - started_at:.3f}"


if __name__ == "__main__":
    main()
