from __future__ import annotations

import os
from uuid import uuid4

from devbox import DevBoxError, Sandbox


def main() -> None:
    template = os.getenv("DEVBOX_TEST_TEMPLATE", "default").strip() or "default"
    print(f"DevBox SDK demo | template={template}")

    with Sandbox.create(
        template,
        timeout=300,
        metadata={"example": "python-sdk-demo"},
        request_timeout=120,
    ) as sandbox:
        print(f"[1/5] Sandbox created | id={sandbox.sandbox_id}")

        info = sandbox.get_info()
        print(f"[2/5] Sandbox ready | state={info.state.value}")

        result = sandbox.commands.run("printf 'hello from DevBox'")
        print(f"[3/5] Command completed | stdout={result.stdout!r}")

        directory = f"/tmp/devbox-demo-{uuid4().hex[:8]}"
        path = f"{directory}/message.txt"
        sandbox.files.make_dir(directory)
        sandbox.files.write(path, "hello from the SDK")
        content = sandbox.files.read(path)
        entries = ", ".join(entry.name for entry in sandbox.files.list(directory))
        print(f"[4/5] Filesystem verified | content={content!r}, entries={entries}")

        process = sandbox.commands.run("cat", background=True, stdin=True, timeout=10)
        process.send_stdin("background input received\n")
        process.close_stdin()
        background = process.wait()
        print(
            f"[5/5] Background process completed | "
            f"pid={background.pid}, stdout={background.stdout.rstrip()!r}"
        )

    print("DevBox SDK demo completed | sandbox deleted")


if __name__ == "__main__":
    try:
        main()
    except DevBoxError as error:
        raise SystemExit(f"DevBox SDK demo failed | {error}") from None
