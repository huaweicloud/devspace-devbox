from __future__ import annotations

import base64
import json
from collections.abc import Callable, Generator, Iterator, Mapping
from typing import Any, Literal, cast

import httpx
import pytest

from devbox import CommandExitError, CommandResult, ProtocolError
from devbox._transport import AsyncTransport, SyncTransport
from devbox.commands import AsyncCommands, CommandHandle, Commands


def test_command_uses_envd_connect_protocol() -> None:
    requests: list[httpx.Request] = []

    def handler(request: httpx.Request) -> httpx.Response:
        requests.append(request)
        assert request.url.path == "/process.Process/Start"
        body = _request_frame(request)
        assert body == {
            "process": {
                "cmd": "/bin/bash",
                "args": ["-l", "-c", "echo hello"],
                "envs": {"LANG": "C"},
                "cwd": "/tmp",
            },
            "stdin": False,
        }
        return _stream_response(
            {"event": {"start": {"pid": 42}}},
            {"event": {"data": {"stdout": _encoded("hello ")}}},
            {"event": {"data": {"stderr": _encoded("warning\n")}}},
            {"event": {"data": {"stdout": _encoded("world\n")}}},
            {"event": {"end": {"exitCode": 0, "exited": True}}},
        )

    output: list[str] = []
    with _transport(handler) as transport:
        result = Commands(lambda: transport).run(
            "echo hello", envs={"LANG": "C"}, cwd="/tmp", on_stdout=output.append
        )

    assert isinstance(result, CommandResult)
    assert result == CommandResult(exit_code=0, stdout="hello world\n", stderr="warning\n", pid=42)
    assert output == ["hello ", "world\n"]
    assert requests[0].headers["Content-Type"] == "application/connect+json"
    assert requests[0].headers["Connect-Protocol-Version"] == "1"


def test_nonzero_command_raises_with_result() -> None:
    def handler(request: httpx.Request) -> httpx.Response:
        return _stream_response(
            {"event": {"start": {"pid": 7}}},
            {"event": {"end": {"exitCode": 7, "exited": True}}},
        )

    with _transport(handler) as transport, pytest.raises(CommandExitError) as raised:
        Commands(lambda: transport).run("exit 7")

    assert raised.value.result.exit_code == 7


def test_command_keeps_stdout_and_stderr_from_the_same_event() -> None:
    def handler(request: httpx.Request) -> httpx.Response:
        return _stream_response(
            {"event": {"start": {"pid": 7}}},
            {
                "event": {
                    "data": {
                        "stdout": _encoded("output"),
                        "stderr": _encoded("warning"),
                    }
                }
            },
            {"event": {"end": {"exitCode": 0}}},
        )

    with _transport(handler) as transport:
        result = Commands(lambda: transport).run("echo output")

    assert result.stdout == "output"
    assert result.stderr == "warning"


def test_command_closes_stream_when_output_callback_fails() -> None:
    closed = False

    def events() -> Generator[Mapping[str, Any], None, None]:
        nonlocal closed
        try:
            yield {"event": {"data": {"stdout": _encoded("output")}}}
        finally:
            closed = True

    def fail(chunk: str) -> None:
        raise RuntimeError(chunk)

    def unused_transport() -> SyncTransport:
        raise AssertionError("unused")

    commands = Commands(unused_transport)
    handle = CommandHandle(7, commands, events())
    with pytest.raises(RuntimeError, match="output"):
        handle.wait(on_stdout=fail)

    assert closed is True


def test_command_closes_stream_when_start_handshake_is_invalid() -> None:
    stream = TrackingStream(_stream_body({"event": {"data": {"stdout": _encoded("early")}}}))

    def handler(request: httpx.Request) -> httpx.Response:
        return httpx.Response(
            200,
            stream=stream,
            headers={"Content-Type": "application/connect+json"},
        )

    with _transport(handler) as transport, pytest.raises(ProtocolError, match="start process"):
        Commands(lambda: transport).run("echo output")

    assert stream.closed is True


def test_background_command_keeps_stream_and_sends_base64_input() -> None:
    requests: list[httpx.Request] = []

    def handler(request: httpx.Request) -> httpx.Response:
        requests.append(request)
        if request.url.path.endswith("/Start"):
            return _stream_response(
                {"event": {"start": {"pid": 42}}},
                {"event": {"end": {"exited": True}}},
            )
        return httpx.Response(200, json={}, headers={"Content-Type": "application/json"})

    with _transport(handler) as transport:
        handle = Commands(lambda: transport).run("cat", background=True, stdin=True)
        assert isinstance(handle, CommandHandle)
        handle.send_stdin("ready\n")
        result = handle.wait()

    assert result.exit_code == 0
    assert requests[1].url.path == "/process.Process/SendInput"
    assert json.loads(requests[1].content) == {
        "process": {"pid": 42},
        "input": {"stdin": _encoded("ready\n")},
    }


def test_completed_command_rejects_further_control_operations() -> None:
    def handler(request: httpx.Request) -> httpx.Response:
        return _stream_response(
            {"event": {"start": {"pid": 42}}},
            {"event": {"end": {"exitCode": 0, "exited": True}}},
        )

    with _transport(handler) as transport:
        handle = Commands(lambda: transport).run("true", background=True)
        handle.wait()

        operations: tuple[Callable[[], None], ...] = (
            lambda: handle.send_stdin("late"),
            handle.close_stdin,
            lambda: handle.send_signal("SIGTERM"),
            handle.kill,
        )
        for operation in operations:
            with pytest.raises(RuntimeError, match="already completed"):
                operation()


def test_background_callbacks_are_attached_when_waiting() -> None:
    with _transport(lambda request: httpx.Response(500)) as transport:
        commands = Commands(lambda: transport)
        with pytest.raises(ValueError, match=r"handle\.wait"):
            commands.run(  # type: ignore[call-overload]
                "echo ready", background=True, on_stdout=lambda chunk: None
            )


def test_rejects_unknown_signal() -> None:
    with (
        _transport(lambda request: httpx.Response(200, json={})) as transport,
        pytest.raises(ValueError, match="SIGTERM or SIGKILL"),
    ):
        invalid = cast(Literal["SIGTERM", "SIGKILL"], "SIGNAL_UNKNOWN")
        Commands(lambda: transport).send_signal(42, invalid)


@pytest.mark.asyncio
async def test_async_command_uses_the_same_protocol() -> None:
    async def handler(request: httpx.Request) -> httpx.Response:
        return _stream_response(
            {"event": {"start": {"pid": 8}}},
            {
                "event": {
                    "data": {
                        "stdout": _encoded("async"),
                        "stderr": _encoded(" warning"),
                    }
                }
            },
            {"event": {"end": {"exited": True}}},
        )

    transport = AsyncTransport(
        "https://envd.test",
        headers={},
        timeout=30,
        transport=httpx.MockTransport(handler),
    )

    async def provide() -> AsyncTransport:
        return transport

    try:
        output: list[str] = []
        errors: list[str] = []
        result = await AsyncCommands(provide).run(
            "echo async", on_stdout=output.append, on_stderr=errors.append
        )
    finally:
        await transport.close()

    assert isinstance(result, CommandResult)
    assert result.stdout == "async"
    assert result.stderr == " warning"
    assert output == ["async"]
    assert errors == [" warning"]


@pytest.mark.asyncio
async def test_completed_async_command_rejects_further_input() -> None:
    async def handler(request: httpx.Request) -> httpx.Response:
        return _stream_response(
            {"event": {"start": {"pid": 8}}},
            {"event": {"end": {"exitCode": 0, "exited": True}}},
        )

    transport = AsyncTransport(
        "https://envd.test",
        headers={},
        timeout=30,
        transport=httpx.MockTransport(handler),
    )

    async def provide() -> AsyncTransport:
        return transport

    try:
        handle = await AsyncCommands(provide).run("true", background=True)
        await handle.wait()
        with pytest.raises(RuntimeError, match="already completed"):
            await handle.send_stdin("late")
    finally:
        await transport.close()


def _transport(handler: Any) -> SyncTransport:
    return SyncTransport(
        "https://envd.test",
        headers={},
        timeout=30,
        transport=httpx.MockTransport(handler),
    )


def _request_frame(request: httpx.Request) -> Mapping[str, Any]:
    content = request.content
    assert content[0] == 0
    size = int.from_bytes(content[1:5], "big")
    assert size == len(content) - 5
    value = json.loads(content[5:])
    assert isinstance(value, Mapping)
    return value


def _stream_response(*events: Mapping[str, Any]) -> httpx.Response:
    return httpx.Response(
        200,
        content=_stream_body(*events),
        headers={"Content-Type": "application/connect+json"},
    )


def _stream_body(*events: Mapping[str, Any]) -> bytes:
    return b"".join(_frame(event) for event in events) + _frame({}, flags=2)


def _frame(value: Mapping[str, Any], *, flags: int = 0) -> bytes:
    data = json.dumps(value, separators=(",", ":")).encode()
    return bytes([flags]) + len(data).to_bytes(4, "big") + data


def _encoded(value: str) -> str:
    return base64.b64encode(value.encode()).decode()


class TrackingStream(httpx.SyncByteStream):
    def __init__(self, content: bytes) -> None:
        self.content = content
        self.closed = False

    def __iter__(self) -> Iterator[bytes]:
        yield self.content

    def close(self) -> None:
        self.closed = True
