from __future__ import annotations

import httpx
import pytest

from devbox import AsyncDevBox, DevBox

TEMPLATES = [
    {
        "templateID": "tpl-devbox-base-test",
        "name": "devbox-base-test",
        "aliases": ["default"],
        "buildID": "devbox-base-test-aarch64-20261012032308",
        "cpuCount": 2,
        "memoryMB": 2500,
        "diskSizeMB": 2040,
    }
]


def test_list_templates() -> None:
    def handler(request: httpx.Request) -> httpx.Response:
        assert request.method == "GET"
        assert request.url.path == "/templates"
        return httpx.Response(200, json=TEMPLATES)

    with DevBox(
        api_key="secret",
        api_url="https://api.test",
        http_transport=httpx.MockTransport(handler),
    ) as client:
        template = client.templates.list()[0]
        assert template.template_id == "tpl-devbox-base-test"
        assert template.aliases == ("default",)
        assert template.memory_mb == 2500


@pytest.mark.asyncio
async def test_list_templates_async() -> None:
    async with AsyncDevBox(
        api_key="secret",
        api_url="https://api.test",
        http_transport=httpx.MockTransport(lambda _: httpx.Response(200, json=TEMPLATES)),
    ) as client:
        template = (await client.templates.list())[0]
        assert template.build_id == "devbox-base-test-aarch64-20261012032308"
