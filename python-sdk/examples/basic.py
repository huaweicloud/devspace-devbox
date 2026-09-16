import os

from devbox import Sandbox


def main() -> None:
    template = os.getenv("DEVBOX_TEST_TEMPLATE", "default")
    with Sandbox.create(template, timeout=300) as sandbox:
        print(f"sandbox created: {sandbox.sandbox_id}")

        result = sandbox.commands.run("printf 'hello from DevBox'")
        print(f"command output: {result.stdout}")

        path = "/tmp/hello.txt"
        sandbox.files.write(path, "hello from the SDK")
        print(f"file content: {sandbox.files.read(path)}")


if __name__ == "__main__":
    main()
