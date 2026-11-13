#!/usr/bin/env python3
"""Direct-mode reply with DeepSeek web search: assert the answer ends with real
web source links and produces no knowledge-base references."""

import argparse
import json
import re
import sys
import time
import urllib.request
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
SOURCE_URL_RE = re.compile(r"\]\((https?://[^)\s]+)\)")


def load_env(path):
    values = {}
    if path.exists():
        for line in path.read_text(encoding="utf-8").splitlines():
            line = line.strip()
            if line and not line.startswith("#") and "=" in line:
                key, value = line.split("=", 1)
                values[key.strip()] = value.strip().strip("\"'")
    return values


def request_json(base, method, path, token=None, payload=None, timeout=60):
    headers = {"Accept": "application/json"}
    data = None
    if payload is not None:
        data = json.dumps(payload, ensure_ascii=False).encode("utf-8")
        headers["Content-Type"] = "application/json"
    if token:
        headers["Authorization"] = "Bearer " + token
    req = urllib.request.Request(base.rstrip("/") + path, data=data, headers=headers, method=method)
    with urllib.request.urlopen(req, timeout=timeout) as resp:
        return json.loads(resp.read().decode("utf-8"))


def stream_direct(base, token, payload, timeout):
    req = urllib.request.Request(
        base.rstrip("/") + "/api/v1/knowledge-chat/" + payload.pop("session_id"),
        data=json.dumps(payload, ensure_ascii=False).encode("utf-8"),
        headers={"Content-Type": "application/json", "Authorization": "Bearer " + token},
        method="POST",
    )
    answer, thinking, refs, errors = [], [], 0, []
    with urllib.request.urlopen(req, timeout=timeout) as resp:
        for raw in resp:
            line = raw.decode("utf-8", "replace").strip()
            if not line.startswith("data:"):
                continue
            try:
                event = json.loads(line[5:].strip())
            except json.JSONDecodeError:
                continue
            kind = event.get("response_type")
            if kind == "answer":
                answer.append(event.get("content") or "")
            elif kind == "thinking":
                thinking.append(event.get("content") or "")
            elif kind == "references":
                refs += 1
            elif kind == "error":
                errors.append(event.get("content") or "error")
            elif kind == "complete":
                break
    return "".join(answer), "".join(thinking), refs, errors


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--env-file", type=Path, default=ROOT / "deployments/.env")
    parser.add_argument("--base-url", default="http://localhost:6152")
    parser.add_argument("--query", default="DeepSeek 最近发布了哪些模型？请给出来源。")
    parser.add_argument("--timeout", type=int, default=180)
    parser.add_argument("--out", type=Path)
    args = parser.parse_args()

    env = load_env(args.env_file)
    login = request_json(args.base_url, "POST", "/api/v1/auth/login", payload={
        "email": env.get("OWNER_EMAIL", ""), "password": env.get("OWNER_PASSWORD", ""),
    })
    token = login.get("token") or (login.get("data") or {}).get("token")
    if not token:
        raise SystemExit("login failed")
    session = request_json(args.base_url, "POST", "/api/v1/sessions", token=token, payload={"title": "accept-direct-websearch"})
    session_id = (session.get("data") or {}).get("id")
    started = time.time()
    answer, thinking, refs, errors = stream_direct(args.base_url, token, {
        "session_id": session_id,
        "query": args.query,
        "retrieval_mode": "direct",
        "web_search_enabled": True,
        "channel": "api",
    }, args.timeout)
    try:
        request_json(args.base_url, "DELETE", f"/api/v1/sessions/{session_id}", token=token)
    except Exception as exc:  # noqa: BLE001
        print(f"warn: session cleanup failed: {exc}", file=sys.stderr)

    source_block = answer.rsplit("**网页来源**", 1)[1] if "**网页来源**" in answer else ""
    urls = SOURCE_URL_RE.findall(source_block)
    problems = []
    if errors:
        problems.append("SSE error: " + "; ".join(errors)[:300])
    if "网页来源" not in answer:
        problems.append("missing 网页来源 block")
    if len(urls) < 3:
        problems.append(f"only {len(urls)} source URLs")
    if len(urls) != len(set(urls)):
        problems.append("duplicate source URLs in 网页来源 block")
    if refs:
        problems.append("direct mode produced knowledge references")
    if not thinking.strip():
        problems.append("direct mode produced no thinking output")
    report = {
        "ok": not problems,
        "problems": problems,
        "elapsed_s": round(time.time() - started, 1),
        "answer_chars": len(answer),
        "thinking_chars": len(thinking),
        "urls": urls[:15],
    }
    print(json.dumps(report, ensure_ascii=False, indent=2))
    if args.out:
        args.out.write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    return 0 if report["ok"] else 1


if __name__ == "__main__":
    sys.exit(main())
