from client import GazeClient, normalize_kb_entries


def test_normalize_kb_entries_passes_through_owned_rows():
    resp = {
        "success": True,
        "data": [{"id": "kb-owned-1", "name": "My Docs"}],
    }
    assert normalize_kb_entries(resp) == [{"id": "kb-owned-1", "name": "My Docs"}]


def test_resolve_kb_id_accepts_name(monkeypatch):
    client = GazeClient("http://example.test/api/v1", "")

    def fake_request(method, path, **kwargs):
        if path == "/knowledge-bases":
            return {
                "success": True,
                "data": [{"id": "kb-owned-1", "name": "技术文档库"}],
            }
        raise AssertionError(f"unexpected request: {method} {path}")

    monkeypatch.setattr(client, "_request", fake_request)

    assert client.resolve_kb_id("技术文档库") == "kb-owned-1"


def test_resolve_kb_id_matches_case_insensitively(monkeypatch):
    client = GazeClient("http://example.test/api/v1", "")

    def fake_request(method, path, **kwargs):
        if path == "/knowledge-bases":
            return {
                "success": True,
                "data": [{"id": "kb-owned-1", "name": "My Docs"}],
            }
        raise AssertionError(f"unexpected request: {method} {path}")

    monkeypatch.setattr(client, "_request", fake_request)

    assert client.resolve_kb_id("my docs") == "kb-owned-1"
