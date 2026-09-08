from __future__ import annotations

import os
import queue
import tempfile
import threading
import time
from collections.abc import Callable, Mapping
from pathlib import Path
from typing import TypeVar
from uuid import uuid4

from devbox import CommandResult, DevBox, DevBoxError, NetworkConfig, PtySize, Sandbox

T = TypeVar("T")


class Validator:
    def __init__(self) -> None:
        self.failures: list[str] = []
        self.tests = 0
        self.started_at = time.monotonic()
        self._output_lock = threading.Lock()

    def verify(
        self,
        name: str,
        operation: Callable[[], T],
        *,
        detail: str = "",
        summarize: Callable[[T], str] | None = None,
    ) -> T | None:
        self.tests += 1
        suffix = f" | {detail}" if detail else ""
        self._log(f"\nTEST   {self.tests:02d} {name}{suffix}")
        started_at = time.monotonic()
        try:
            result = operation()
        except Exception as error:
            self.failures.append(name)
            elapsed = time.monotonic() - started_at
            self._log(f"FAIL   {name} | {elapsed:.3f}s | {type(error).__name__}: {error}")
            return None
        elapsed = time.monotonic() - started_at
        summary = summarize(result) if summarize is not None else "validated"
        suffix = f" | {summary}" if summary else ""
        self._log(f"PASS   {name} | {elapsed:.3f}s{suffix}")
        return result

    def action(
        self,
        name: str,
        operation: Callable[[], T],
        *,
        detail: str = "",
        summarize: Callable[[T], str] | None = None,
    ) -> T:
        suffix = f" | {detail}" if detail else ""
        self._log(f"  CALL   {name}{suffix}")
        started_at = time.monotonic()
        try:
            result = operation()
        except Exception as error:
            elapsed = time.monotonic() - started_at
            self._log(f"  ERROR  {name} | {elapsed:.3f}s | {type(error).__name__}: {error}")
            raise
        elapsed = time.monotonic() - started_at
        summary = summarize(result) if summarize is not None else _summarize(result)
        suffix = f" | {summary}" if summary else ""
        self._log(f"  RETURN {name} | {elapsed:.3f}s{suffix}")
        return result

    def note(self, message: str) -> None:
        self._log(f"  INFO   {message}")

    def section(self, name: str) -> None:
        self._log(f"\n{'-' * 24} {name} {'-' * 24}")

    def skip(self, name: str, reason: str) -> None:
        self._log(f"\nSKIP   {name} | {reason}")

    def finish(self) -> None:
        elapsed = time.monotonic() - self.started_at
        passed = self.tests - len(self.failures)
        lines = [
            "\n" + "=" * 72,
            f"SUMMARY tests={self.tests} passed={passed} failed={len(self.failures)} "
            f"elapsed={elapsed:.3f}s",
        ]
        if self.failures:
            lines.append(f"FAILED  {', '.join(self.failures)}")
        lines.append("=" * 72)
        self._log("\n".join(lines))

    def _log(self, message: str) -> None:
        with self._output_lock:
            print(message, flush=True)


def main() -> None:
    template = os.getenv("DEVBOX_TEST_TEMPLATE", "default").strip() or "default"
    validator = Validator()

    print("=" * 72)
    print("DevBox SDK full validation")
    print(f"template={template!r}")
    print("scope=sandbox lifecycle, commands, filesystem, PTY, Git")
    print("The temporary sandbox and generated test files are removed at the end.")
    print("=" * 72, flush=True)

    with DevBox() as client:
        sandbox = validator.verify(
            "sandbox.create",
            lambda: client.sandboxes.create(
                template,
                timeout=300,
                metadata={"sdk_validation": "full"},
                network=NetworkConfig(allow_internet_access=True),
            ),
            detail=f"template={template!r}, timeout=300s, internet_access=true",
            summarize=lambda value: f"sandbox_id={value.sandbox_id}, state={value.info.state}",
        )
        if sandbox is None:
            validator.finish()
            raise SystemExit("Full validation stopped because sandbox creation failed")

        validator.note(f"sandbox ready | sandbox_id={sandbox.sandbox_id}")
        try:
            validator.section("Manager lifecycle")
            validate_manager(client, sandbox, validator)
            validator.section("Runtime gateway")
            runtime_ready = validator.verify(
                "runtime.ready",
                lambda: _validate_runtime_ready(sandbox, validator),
                detail="execute a command through the runtime gateway",
            )
            if runtime_ready is None:
                for area in ("commands", "filesystem", "pty", "git"):
                    validator.skip(area, "runtime gateway is unavailable")
            else:
                validator.section("Commands")
                validate_commands(sandbox, validator)
                validator.section("Filesystem")
                validate_filesystem(sandbox, validator)
                validator.section("PTY")
                validate_pty(sandbox, validator)
                validator.section("Git")
                validate_git(sandbox, validator)
        finally:
            validator.section("Cleanup")
            validator.verify(
                "sandbox.delete",
                sandbox.kill,
                detail=f"sandbox_id={sandbox.sandbox_id}",
            )
            validator.action("sandbox.close", sandbox.close, detail="close local transports")

    validator.finish()
    if validator.failures:
        names = ", ".join(validator.failures)
        raise SystemExit(f"Full validation failed ({len(validator.failures)}): {names}")
    print("DevBox phase-one full validation passed")


def validate_manager(client: DevBox, sandbox: Sandbox, validator: Validator) -> None:
    validator.verify(
        "manager.get",
        lambda: _expect_id(
            validator.action("sandbox.get_info", sandbox.get_info), sandbox.sandbox_id
        ),
        detail=f"sandbox_id={sandbox.sandbox_id}",
    )
    validator.verify(
        "manager.is_running",
        lambda: _expect(validator.action("sandbox.is_running", sandbox.is_running), "not running"),
    )
    validator.verify(
        "manager.list",
        lambda: _validate_manager_list(client, sandbox, validator),
        detail="created sandbox must be present",
    )
    validator.verify(
        "manager.set_timeout",
        lambda: validator.action(
            "sandbox.set_timeout", lambda: sandbox.set_timeout(300), detail="timeout=300s"
        ),
    )
    validator.verify(
        "manager.refresh",
        lambda: validator.action(
            "sandbox.refresh", lambda: sandbox.refresh(300), detail="duration=300s"
        ),
    )
    validator.verify(
        "manager.metrics",
        lambda: validator.action("sandbox.get_metrics", sandbox.get_metrics),
    )
    validator.verify(
        "manager.aggregate_metrics",
        lambda: validator.action(
            "client.sandboxes.metrics",
            lambda: client.sandboxes.metrics([sandbox.sandbox_id]),
            detail=f"sandbox_ids=[{sandbox.sandbox_id}]",
        ),
    )
    validator.verify(
        "manager.logs",
        lambda: validator.action(
            "sandbox.get_logs", lambda: sandbox.get_logs(limit=20), detail="limit=20"
        ),
    )
    validator.verify(
        "manager.update_network",
        lambda: validator.action(
            "sandbox.update_network",
            lambda: sandbox.update_network(NetworkConfig(allow_internet_access=True)),
            detail="allow_internet_access=true",
        ),
    )


def _validate_manager_list(client: DevBox, sandbox: Sandbox, validator: Validator) -> None:
    page = validator.action(
        "client.sandboxes.list",
        lambda: client.sandboxes.list(limit=100),
        detail="limit=100",
        summarize=lambda value: (
            f"items={len(value.items)}, next_token={value.next_token!r}, "
            f"total_running={value.total!r}"
        ),
    )
    _expect(
        sandbox.sandbox_id in {item.sandbox_id for item in page.items},
        "created sandbox is missing from list",
    )


def validate_commands(sandbox: Sandbox, validator: Validator) -> None:
    validator.verify(
        "commands.foreground",
        lambda: _validate_foreground(sandbox, validator),
        detail="run command and collect final output",
    )
    validator.verify(
        "commands.options",
        lambda: _validate_command_options(sandbox, validator),
        detail="cwd, environment, stdout callback, stderr callback",
    )
    validator.verify(
        "commands.background",
        lambda: _validate_background(sandbox, validator),
        detail="start -> list -> wait",
    )
    validator.verify(
        "commands.stdin",
        lambda: _validate_stdin(sandbox, validator),
        detail="start -> send stdin -> close stdin -> wait",
    )
    validator.verify(
        "commands.signal",
        lambda: _validate_signal(sandbox, validator),
        detail="start -> SIGTERM -> wait",
    )
    validator.verify(
        "commands.reconnect",
        lambda: _validate_reconnect(sandbox, validator),
        detail="start -> disconnect -> reconnect -> release -> wait",
    )


def _validate_runtime_ready(sandbox: Sandbox, validator: Validator) -> bool:
    result = validator.action(
        "commands.run",
        lambda: sandbox.commands.run("printf runtime-ready"),
        detail="runtime gateway probe",
    )
    _expect_result(result, stdout="runtime-ready")
    return True


def _validate_foreground(sandbox: Sandbox, validator: Validator) -> None:
    result = validator.action(
        "commands.run",
        lambda: sandbox.commands.run("printf command-ok"),
        detail="foreground command='printf command-ok'",
    )
    _expect_result(result, stdout="command-ok")


def _validate_command_options(sandbox: Sandbox, validator: Validator) -> None:
    stdout_chunks: list[str] = []
    stderr_chunks: list[str] = []
    result = validator.action(
        "commands.run",
        lambda: sandbox.commands.run(
            'printf "$DEVBOX_FULL_TEST"; printf error-ok >&2',
            cwd="/tmp",
            envs={"DEVBOX_FULL_TEST": "env-ok"},
            on_stdout=stdout_chunks.append,
            on_stderr=stderr_chunks.append,
        ),
        detail="cwd=/tmp, env=DEVBOX_FULL_TEST, callbacks=enabled",
    )
    _expect_result(result, stdout="env-ok", stderr="error-ok")
    _expect("".join(stdout_chunks) == "env-ok", "stdout callback content differs")
    _expect("".join(stderr_chunks) == "error-ok", "stderr callback content differs")
    validator.note(
        f"callbacks received | stdout_chunks={len(stdout_chunks)}, "
        f"stderr_chunks={len(stderr_chunks)}"
    )


def _validate_background(sandbox: Sandbox, validator: Validator) -> None:
    handle = validator.action(
        "commands.run",
        lambda: sandbox.commands.run("sleep 1; printf background-ok", background=True, timeout=10),
        detail="background=true, timeout=10s",
        summarize=lambda value: f"pid={value.pid}",
    )
    processes = validator.action(
        "commands.list",
        sandbox.commands.list,
        summarize=lambda value: f"processes={len(value)}, pids={[item.pid for item in value]}",
    )
    _expect(handle.pid in {process.pid for process in processes}, "process is missing from list")
    result = validator.action("command_handle.wait", handle.wait, detail=f"pid={handle.pid}")
    _expect_result(result, stdout="background-ok")


def _validate_stdin(sandbox: Sandbox, validator: Validator) -> None:
    handle = validator.action(
        "commands.run",
        lambda: sandbox.commands.run("cat", background=True, stdin=True, timeout=10),
        detail="command='cat', background=true, stdin=true",
        summarize=lambda value: f"pid={value.pid}",
    )
    validator.action(
        "command_handle.send_stdin",
        lambda: handle.send_stdin("stdin-ok\n"),
        detail=f"pid={handle.pid}, bytes=9",
    )
    validator.action("command_handle.close_stdin", handle.close_stdin, detail=f"pid={handle.pid}")
    result = validator.action("command_handle.wait", handle.wait, detail=f"pid={handle.pid}")
    _expect_result(result, stdout="stdin-ok\n")


def _validate_signal(sandbox: Sandbox, validator: Validator) -> None:
    handle = validator.action(
        "commands.run",
        lambda: sandbox.commands.run("sleep 30", background=True, timeout=35),
        detail="background=true, timeout=35s",
        summarize=lambda value: f"pid={value.pid}",
    )
    validator.action(
        "command_handle.send_signal",
        lambda: handle.send_signal("SIGTERM"),
        detail=f"pid={handle.pid}, signal=SIGTERM",
    )
    result = validator.action(
        "command_handle.wait",
        lambda: handle.wait(check=False),
        detail=f"pid={handle.pid}, check=false",
    )
    _expect(result.exit_code != 0, "signalled process exited successfully")


def _validate_reconnect(sandbox: Sandbox, validator: Validator) -> None:
    marker = f"/tmp/devbox-sdk-reconnect-{uuid4().hex}"
    handle = validator.action(
        "commands.run",
        lambda: sandbox.commands.run(
            f"while [ ! -f {marker} ]; do sleep 0.05; done; printf reconnect-ok",
            background=True,
            timeout=10,
        ),
        detail=f"background=true, marker={marker}",
        summarize=lambda value: f"pid={value.pid}",
    )
    pid = handle.pid
    validator.action("command_handle.disconnect", handle.disconnect, detail=f"pid={pid}")
    validator.note("wait 0.2s so the original stream is fully detached")
    time.sleep(0.2)
    connected = validator.action(
        "commands.connect",
        lambda: sandbox.commands.connect(pid, timeout=10),
        detail=f"pid={pid}, timeout=10s",
        summarize=lambda value: f"pid={value.pid}",
    )
    responses: queue.Queue[CommandResult | Exception] = queue.Queue(maxsize=1)

    def consume() -> None:
        try:
            responses.put(
                validator.action(
                    "command_handle.wait", connected.wait, detail=f"reconnected pid={pid}"
                )
            )
        except Exception as error:
            responses.put(error)

    try:
        thread = threading.Thread(target=consume, daemon=True, name="devbox-command-reconnect")
        thread.start()
        validator.note("reconnected output consumer started; wait 0.2s before releasing process")
        time.sleep(0.2)
        validator.action(
            "files.write",
            lambda: sandbox.files.write(marker, "ready"),
            detail=f"release process marker={marker}",
        )
        result = _queue_result(
            responses,
            timeout=10,
            timeout_message="reconnected command did not finish",
        )
        _expect_result(result, stdout="reconnect-ok")
    finally:
        if sandbox.files.exists(marker):
            validator.action("files.remove", lambda: sandbox.files.remove(marker), detail=marker)


def validate_filesystem(sandbox: Sandbox, validator: Validator) -> None:
    root = f"/tmp/devbox-sdk-full-{uuid4().hex[:8]}"
    validator.note(f"filesystem workspace | root={root}")
    validator.verify(
        "filesystem.basic",
        lambda: _validate_filesystem_basic(sandbox, root, validator),
        detail="mkdir -> batch write -> read -> stat -> list -> move",
    )
    validator.verify(
        "filesystem.transfer",
        lambda: _validate_transfer(sandbox, root, validator),
        detail="local upload -> remote file -> local download",
    )
    validator.verify(
        "filesystem.watch",
        lambda: _validate_watch(sandbox, root, validator),
        detail="start watcher -> modify directory -> receive event",
    )
    validator.verify(
        "filesystem.remove",
        lambda: validator.action(
            "files.remove", lambda: sandbox.files.remove(root), detail=f"recursive path={root}"
        ),
    )


def _validate_filesystem_basic(sandbox: Sandbox, root: str, validator: Validator) -> None:
    validator.action("files.make_dir", lambda: sandbox.files.make_dir(root), detail=root)
    text_path = f"{root}/message.txt"
    binary_path = f"{root}/data.bin"
    validator.action(
        "files.write_batch",
        lambda: sandbox.files.write_batch({text_path: "file-ok", binary_path: b"\x00\x01\x02"}),
        detail=f"files=[{text_path}, {binary_path}]",
    )
    text = validator.action("files.read", lambda: sandbox.files.read(text_path), detail=text_path)
    _expect(text == "file-ok", "text content differs")
    binary = validator.action(
        "files.read_bytes", lambda: sandbox.files.read_bytes(binary_path), detail=binary_path
    )
    _expect(binary == b"\x00\x01\x02", "binary content differs")
    exists = validator.action(
        "files.exists", lambda: sandbox.files.exists(text_path), detail=text_path
    )
    _expect(exists, "written file does not exist")
    info = validator.action("files.stat", lambda: sandbox.files.stat(text_path), detail=text_path)
    _expect(info.size == 7, "file size differs")
    entries = validator.action(
        "files.list",
        lambda: sandbox.files.list(root, depth=2),
        detail=f"path={root}, depth=2",
        summarize=lambda value: f"entries={len(value)}, names={[item.name for item in value]}",
    )
    _expect(len(entries) >= 2, "directory listing is incomplete")
    moved_path = f"{root}/moved.txt"
    validator.action(
        "files.move",
        lambda: sandbox.files.move(text_path, moved_path),
        detail=f"{text_path} -> {moved_path}",
    )
    destination_exists = validator.action(
        "files.exists", lambda: sandbox.files.exists(moved_path), detail=moved_path
    )
    source_exists = validator.action(
        "files.exists", lambda: sandbox.files.exists(text_path), detail=text_path
    )
    _expect(destination_exists, "moved file does not exist")
    _expect(not source_exists, "source still exists after move")
    validator.note("move verified | destination exists and source no longer exists")


def _validate_transfer(sandbox: Sandbox, root: str, validator: Validator) -> None:
    with tempfile.TemporaryDirectory() as local_directory:
        source = Path(local_directory, "upload.txt")
        destination = Path(local_directory, "download.txt")
        source.write_text("transfer-ok", encoding="utf-8")
        validator.note(f"local fixture created | path={source}, bytes={source.stat().st_size}")
        validator.action(
            "files.upload",
            lambda: sandbox.files.upload(source, f"{root}/upload.txt"),
            detail=f"{source} -> {root}/upload.txt",
        )
        validator.action(
            "files.download",
            lambda: sandbox.files.download(f"{root}/upload.txt", destination),
            detail=f"{root}/upload.txt -> {destination}",
        )
        _expect(destination.read_text(encoding="utf-8") == "transfer-ok", "download differs")
        validator.note("download content matches uploaded content")


def _validate_watch(sandbox: Sandbox, root: str, validator: Validator) -> None:
    responses: queue.Queue[object] = queue.Queue(maxsize=1)

    def consume() -> None:
        events = None
        try:
            events = validator.action(
                "files.watch", lambda: sandbox.files.watch(root), detail=f"path={root}"
            )
            validator.note("watch stream opened; waiting for the first filesystem event")
            event = next(events)
        except Exception as error:
            responses.put(error)
            return
        finally:
            if events is not None:
                validator.action("files.watch.close", events.close, detail=f"path={root}")
        responses.put(event)

    thread = threading.Thread(target=consume, daemon=True, name="devbox-files-watch")
    thread.start()
    validator.note("wait 0.5s for the watch stream to become ready")
    time.sleep(0.5)
    watch_path = f"{root}/watch.txt"
    validator.action(
        "files.write",
        lambda: sandbox.files.write(watch_path, "watch-ok"),
        detail=f"trigger watcher with path={watch_path}",
    )
    validator.note("waiting up to 10s for a watch event")
    try:
        event = responses.get(timeout=10)
    except queue.Empty as error:
        raise AssertionError("filesystem watcher did not receive an event") from error
    if isinstance(event, Exception):
        raise event
    _expect(isinstance(event, Mapping), "filesystem watcher returned an invalid event")
    validator.note(f"watch event received | {_short_repr(event)}")


def validate_pty(sandbox: Sandbox, validator: Validator) -> None:
    validator.verify(
        "pty.interaction",
        lambda: _validate_pty(sandbox, validator),
        detail="start -> resize -> input -> disconnect -> reconnect -> output",
    )


def _validate_pty(sandbox: Sandbox, validator: Validator) -> None:
    marker = f"/tmp/devbox-sdk-pty-{uuid4().hex}"
    session = validator.action(
        "pty.start",
        lambda: sandbox.pty.start(size=PtySize(rows=24, cols=80)),
        detail="rows=24, cols=80",
        summarize=lambda value: f"pid={value.pid}",
    )
    validator.action(
        "pty.resize",
        lambda: sandbox.pty.resize(session.pid, PtySize(rows=30, cols=100)),
        detail=f"pid={session.pid}, rows=30, cols=100",
    )
    command = f"while [ ! -f {marker} ]; do sleep 0.05; done; printf 'pty-ok\\n'; exit\n"
    validator.action(
        "pty_handle.send_stdin",
        lambda: session.send_stdin(command),
        detail=f"pid={session.pid}, wait marker then print and exit",
    )
    pid = session.pid
    validator.action("pty_handle.disconnect", session.disconnect, detail=f"pid={pid}")
    validator.note("wait 0.2s so the original PTY stream is fully detached")
    time.sleep(0.2)
    reconnected = validator.action(
        "pty.connect",
        lambda: sandbox.pty.connect(pid),
        detail=f"pid={pid}",
        summarize=lambda value: f"pid={value.pid}",
    )
    responses: queue.Queue[CommandResult | Exception] = queue.Queue(maxsize=1)

    def consume() -> None:
        try:
            responses.put(
                validator.action(
                    "pty_handle.wait",
                    lambda: reconnected.wait(check=False),
                    detail=f"pid={pid}, check=false",
                )
            )
        except Exception as error:
            responses.put(error)

    try:
        thread = threading.Thread(target=consume, daemon=True, name="devbox-pty-reconnect")
        thread.start()
        validator.note("reconnected PTY consumer started; wait 0.2s before releasing session")
        time.sleep(0.2)
        validator.action(
            "files.write",
            lambda: sandbox.files.write(marker, "ready"),
            detail=f"release PTY marker={marker}",
        )
        result = _queue_result(
            responses,
            timeout=10,
            timeout_message="reconnected PTY did not finish",
        )
        _expect(result.exit_code == 0, f"PTY exited with status {result.exit_code}")
        _expect("pty-ok" in result.stdout, "PTY output is missing")
    finally:
        if sandbox.files.exists(marker):
            validator.action("files.remove", lambda: sandbox.files.remove(marker), detail=marker)


def validate_git(sandbox: Sandbox, validator: Validator) -> None:
    validator.verify(
        "git.workflow",
        lambda: _validate_git(sandbox, validator),
        detail="init -> commit -> push -> clone -> branch -> pull",
    )


def _validate_git(sandbox: Sandbox, validator: Validator) -> None:
    root = f"/tmp/devbox-sdk-git-{uuid4().hex[:8]}"
    source = f"{root}/source"
    remote = f"{root}/remote.git"
    clone = f"{root}/clone"
    validator.note(f"Git workspace | source={source}, remote={remote}, clone={clone}")
    validator.action(
        "commands.run",
        lambda: sandbox.commands.run(
            f"mkdir -p {root} && git init --bare {remote} && git init -b main {source}"
        ),
        detail="create local source and bare remote repositories",
    )
    validator.action(
        "git.set_config",
        lambda: sandbox.git.set_config(source, "user.name", "DevBox SDK"),
        detail="user.name=DevBox SDK",
    )
    validator.action(
        "git.set_config",
        lambda: sandbox.git.set_config(source, "user.email", "sdk@devbox.local"),
        detail="user.email=sdk@devbox.local",
    )
    validator.action(
        "files.write",
        lambda: sandbox.files.write(f"{source}/README.md", "first\n"),
        detail=f"path={source}/README.md, content='first'",
    )
    validator.action("git.add", lambda: sandbox.git.add(source), detail="paths='.'")
    validator.action(
        "git.commit", lambda: sandbox.git.commit(source, "first"), detail="message='first'"
    )
    validator.action(
        "commands.run",
        lambda: sandbox.commands.run(f"git -C {source} remote add origin {remote}"),
        detail="configure local bare repository as origin",
    )
    validator.action(
        "git.push",
        lambda: sandbox.git.push(source, branch="main"),
        detail="branch=main",
    )
    validator.action(
        "git.clone",
        lambda: sandbox.git.clone(remote, clone, branch="main"),
        detail=f"{remote} -> {clone}, branch=main",
    )
    validator.action(
        "git.checkout",
        lambda: sandbox.git.checkout(clone, "validation", create=True),
        detail="reference=validation, create=true",
    )
    validator.action("git.status", lambda: sandbox.git.status(clone), detail=clone)
    validator.action(
        "files.write",
        lambda: sandbox.files.write(f"{source}/README.md", "second\n"),
        detail=f"path={source}/README.md, content='second'",
    )
    validator.action("git.add", lambda: sandbox.git.add(source), detail="paths='.'")
    validator.action(
        "git.commit", lambda: sandbox.git.commit(source, "second"), detail="message='second'"
    )
    validator.action(
        "git.push",
        lambda: sandbox.git.push(source, branch="main"),
        detail="branch=main",
    )
    validator.action(
        "git.pull",
        lambda: sandbox.git.pull(clone, branch="main"),
        detail="branch=main",
    )
    content = validator.action(
        "files.read", lambda: sandbox.files.read(f"{clone}/README.md"), detail="verify pull"
    )
    _expect(content == "second\n", "git pull did not update")
    validator.action("files.remove", lambda: sandbox.files.remove(root), detail=root)


def _expect_result(
    result: CommandResult,
    *,
    stdout: str | None = None,
    stderr: str | None = None,
) -> None:
    _expect(result.exit_code == 0, f"command exited with status {result.exit_code}")
    if stdout is not None:
        _expect(result.stdout == stdout, f"unexpected stdout: {result.stdout!r}")
    if stderr is not None:
        _expect(result.stderr == stderr, f"unexpected stderr: {result.stderr!r}")


def _expect_id(info: object, sandbox_id: str) -> None:
    _expect(getattr(info, "sandbox_id", None) == sandbox_id, "sandbox ID differs")


def _expect(condition: bool, message: str) -> None:
    if not condition:
        raise AssertionError(message)


def _queue_result(
    responses: queue.Queue[CommandResult | Exception],
    *,
    timeout: float,
    timeout_message: str,
) -> CommandResult:
    try:
        result = responses.get(timeout=timeout)
    except queue.Empty as error:
        raise AssertionError(timeout_message) from error
    if isinstance(result, Exception):
        raise result
    return result


def _summarize(value: object) -> str:
    if value is None:
        return "completed"
    if isinstance(value, CommandResult):
        return (
            f"exit_code={value.exit_code}, stdout={_short_repr(value.stdout)}, "
            f"stderr={_short_repr(value.stderr)}"
        )
    if isinstance(value, bool):
        return f"value={str(value).lower()}"
    if isinstance(value, bytes):
        return f"bytes={len(value)}, value={_short_repr(value)}"
    if isinstance(value, str):
        return f"value={_short_repr(value)}"
    if isinstance(value, Mapping):
        return f"items={len(value)}, value={_short_repr(value)}"
    if isinstance(value, (list, tuple)):
        return f"items={len(value)}, value={_short_repr(value)}"
    sandbox_id = getattr(value, "sandbox_id", None)
    state = getattr(value, "state", None)
    if sandbox_id is not None:
        return f"sandbox_id={sandbox_id}, state={state}"
    pid = getattr(value, "pid", None)
    if pid is not None:
        return f"pid={pid}"
    path = getattr(value, "path", None)
    size = getattr(value, "size", None)
    if path is not None:
        return f"path={path}, size={size!r}"
    return _short_repr(value)


def _short_repr(value: object, limit: int = 240) -> str:
    rendered = repr(value).replace("\n", "\\n")
    if len(rendered) <= limit:
        return rendered
    return f"{rendered[: limit - 3]}..."


if __name__ == "__main__":
    try:
        main()
    except DevBoxError as error:
        raise SystemExit(f"Full validation stopped: {error}") from None
