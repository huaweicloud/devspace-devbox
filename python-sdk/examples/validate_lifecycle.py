"""Run after deploying a Manager with pause/resume support. Creates one sandbox."""

import os
import time
from uuid import uuid4

from devbox import DevBox, SandboxState


def main() -> None:
    marker = uuid4().hex
    path = "/tmp/devbox-lifecycle-proof"
    with DevBox(request_timeout=150) as client:
        sandbox = client.sandboxes.create(
            os.getenv("DEVBOX_TEST_TEMPLATE", "default"),
            timeout=300,
            envs={"LIFECYCLE_MARKER": marker},
        )
        print(f"PASS create | sandbox_id={sandbox.sandbox_id}", flush=True)
        try:
            result = sandbox.commands.run('printf "%s" "$LIFECYCLE_MARKER"')
            assert result.stdout == marker, result.stdout
            sandbox.files.write(path, marker)
            process = sandbox.commands.run("sleep 240", background=True)
            pid = process.pid
            process.disconnect()
            print(f"PASS prepare | env and file marker={marker}, background pid={pid}", flush=True)

            sandbox.set_timeout(600)
            extended = sandbox.get_info().end_at
            assert extended is not None, "Manager did not return a deadline"
            sandbox.refresh(120)
            assert sandbox.get_info().end_at == extended, "refresh shortened the deadline"
            sandbox.set_timeout(300)
            shortened = sandbox.get_info().end_at
            assert shortened is not None and shortened < extended, "set_timeout did not shorten"
            print(f"PASS deadlines | extended={extended}, shortened={shortened}", flush=True)

            started = time.monotonic()
            sandbox.pause()
            assert sandbox.get_info().state is SandboxState.PAUSED
            print(f"PASS pause | {time.monotonic() - started:.3f}s", flush=True)
            sandbox.resume(timeout=300)
            assert sandbox.files.read(path) == marker, "file was not restored"
            sandbox.commands.run(f"kill -0 {pid}")
            print(f"PASS resume | file retained, original pid={pid} still exists", flush=True)

            sandbox.pause()
            sandbox.close()
            sandbox = client.sandboxes.connect(sandbox.sandbox_id, timeout=300)
            assert sandbox.files.read(path) == marker
            print("PASS connect | paused sandbox resumed through Manager connect", flush=True)

            sandbox.set_timeout(5)
            deadline = time.monotonic() + 75
            while sandbox.is_running() and time.monotonic() < deadline:
                time.sleep(2)
            assert not sandbox.is_running(), "expired sandbox was not cleaned up within 75s"
            print("PASS expiry | sandbox no longer running", flush=True)
        finally:
            try:
                sandbox.kill()
                print("CLEANUP | test sandbox deleted", flush=True)
            finally:
                sandbox.close()
    print("Lifecycle validation passed", flush=True)


if __name__ == "__main__":
    main()
