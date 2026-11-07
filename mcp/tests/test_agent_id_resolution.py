from client import GazeClient


def test_resolve_agent_id_accepts_non_uuid_agent_id(monkeypatch):
    client = GazeClient("http://example.test/api/v1", "")
    agents = {
        "data": [
            {
                "id": "builtin-gaze-researcher",
                "name": "维基问答",
            }
        ]
    }
    monkeypatch.setattr(client, "_request", lambda *args, **kwargs: agents)

    assert (
        client.resolve_agent_id("builtin-gaze-researcher")
        == "builtin-gaze-researcher"
    )


def test_resolve_agent_id_accepts_agent_name_case_insensitively(monkeypatch):
    client = GazeClient("http://example.test/api/v1", "")
    agents = {
        "data": [
            {
                "id": "builtin-gaze-researcher",
                "name": "Gaze Questioner",
            }
        ]
    }
    monkeypatch.setattr(client, "_request", lambda *args, **kwargs: agents)

    assert client.resolve_agent_id("gaze questioner") == "builtin-gaze-researcher"
