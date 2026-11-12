#!/usr/bin/env python3
"""Run the RAG study/review 10-question acceptance against a live WellRead stack."""

import argparse
import json
import re
import sys
import time
import urllib.error
import urllib.request
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
DEFAULT_CASES = Path(__file__).with_name("rag_study_review_cases.json")
FORBIDDEN_TERMS = ("支付", "费率", "商户")
IMAGE_RE = re.compile(r"!\[[^\]]*\]\(resource://[^)]+\)")
IMAGE_URL_RE = re.compile(r"!\[[^\]]*\]\((resource://[^)]+)\)")
STUDY_TERMS = ("结论", "背景", "定义", "原理", "易错", "考点")
REVIEW_TERMS = ("考点", "易错", "自测")


def load_env_file(path: Path) -> dict:
    values = {}
    if not path.exists():
        return values
    for line in path.read_text(encoding="utf-8").splitlines():
        line = line.strip()
        if not line or line.startswith("#") or "=" not in line:
            continue
        key, value = line.split("=", 1)
        value = value.strip()
        if len(value) >= 2 and value[0] == value[-1] and value[0] in "\"'":
            value = value[1:-1]
        values[key.strip()] = value
    return values


def request_json(base_url, method, path, token=None, payload=None, timeout=60):
    data = None
    headers = {"Accept": "application/json"}
    if payload is not None:
        data = json.dumps(payload, ensure_ascii=False).encode("utf-8")
        headers["Content-Type"] = "application/json"
    if token:
        headers["Authorization"] = "Bearer " + token
    req = urllib.request.Request(base_url.rstrip("/") + path, data=data, headers=headers, method=method)
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            return json.loads(resp.read().decode("utf-8"))
    except urllib.error.HTTPError as exc:
        body = exc.read().decode("utf-8", "replace")
        raise RuntimeError(f"{method} {path} -> HTTP {exc.code}: {body[:500]}") from exc


def login(base_url, email, password):
    body = request_json(base_url, "POST", "/api/v1/auth/login", payload={"email": email, "password": password})
    token = body.get("token") or (body.get("data") or {}).get("token")
    if not token:
        raise RuntimeError("login response contains no token")
    return token


def resolve_kb(base_url, token, wanted):
    if re.fullmatch(r"[0-9a-fA-F-]{36}", wanted or ""):
        return wanted
    body = request_json(base_url, "GET", "/api/v1/knowledge-bases", token=token)
    for kb in body.get("data") or []:
        if (kb.get("name") or "").strip() == wanted:
            return kb.get("id")
    raise RuntimeError(f"knowledge base not found by name: {wanted}")


def create_session(base_url, token, title):
    body = request_json(base_url, "POST", "/api/v1/sessions", token=token, payload={"title": title})
    session_id = (body.get("data") or {}).get("id")
    if not session_id:
        raise RuntimeError("session response contains no id")
    return session_id


def delete_session(base_url, token, session_id):
    try:
        request_json(base_url, "DELETE", f"/api/v1/sessions/{session_id}", token=token)
    except Exception as exc:  # noqa: BLE001 - cleanup must not mask the test result
        print(f"warn: failed to delete session {session_id}: {exc}", file=sys.stderr)


def stream_qa(base_url, token, session_id, payload, timeout):
    req = urllib.request.Request(
        f"{base_url.rstrip('/')}/api/v1/knowledge-chat/{session_id}",
        data=json.dumps(payload, ensure_ascii=False).encode("utf-8"),
        headers={
            "Accept": "text/event-stream",
            "Content-Type": "application/json",
            "Authorization": "Bearer " + token,
        },
        method="POST",
    )
    parts, references, errors = [], [], []
    with urllib.request.urlopen(req, timeout=timeout) as resp:
        for raw in resp:
            line = raw.decode("utf-8", "replace").strip()
            if not line.startswith("data:"):
                continue
            data = line[5:].strip()
            if not data:
                continue
            try:
                evt = json.loads(data)
            except json.JSONDecodeError:
                continue
            rtype = evt.get("response_type") or ""
            if rtype == "answer":
                parts.append(evt.get("content") or "")
            elif rtype == "references":
                references = evt.get("knowledge_references") or []
            elif rtype == "error":
                errors.append(evt.get("content") or "unknown error")
            elif rtype == "complete":
                break
    return "".join(parts).strip(), references, errors


def evaluate(case, answer, references, errors):
    problems = []
    if errors:
        problems.append("SSE error: " + "; ".join(errors)[:300])
    chars = len(answer)
    if chars < case["min_chars"] or chars > case["max_chars"]:
        problems.append(f"chars {chars} outside {case['min_chars']}-{case['max_chars']}")
    if not answer:
        problems.append("empty answer")
    for term in FORBIDDEN_TERMS:
        if term in answer:
            problems.append(f"forbidden term: {term}")
    if "[[IMG:" in answer:
        problems.append("unreplaced [[IMG:n]] placeholder")

    ref_files = [(r.get("knowledge_filename") or r.get("knowledge_title") or "").strip() for r in references]
    if not references:
        problems.append("no references")
    if any(not name for name in ref_files):
        problems.append("reference without file name")

    if case["kb"] == "image":
        images = len(IMAGE_RE.findall(answer))
        if images < 1:
            problems.append("image answer has no resource:// image")
        if not references:
            problems.append("image answer has no references")
    else:
        terms = STUDY_TERMS if case["mode"] == "study" else REVIEW_TERMS
        hits = [term for term in terms if term in answer]
        minimum = 2 if case["mode"] == "study" else 2
        if len(hits) < minimum:
            problems.append(f"{case['mode']} sections missing (found {hits})")
        if case["mode"] == "review" and "自测" not in answer and "答案" not in answer:
            problems.append("review answer missing self-test")

    return {
        "ok": not problems,
        "problems": problems,
        "chars": chars,
        "refs": len(references),
        "ref_files": ref_files,
        "images": len(IMAGE_RE.findall(answer)),
        "image_urls": IMAGE_URL_RE.findall(answer)[:10],
        "headings": re.findall(r"^#{1,4}\s+(.+)$", answer, flags=re.MULTILINE),
        "has_table": "|" in answer,
    }


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--env-file", type=Path, default=ROOT / "deployments" / ".env")
    parser.add_argument("--base-url", default="http://localhost:6152")
    parser.add_argument("--email")
    parser.add_argument("--password")
    parser.add_argument("--text-kb", default="测试库")
    parser.add_argument("--image-kb", default="图片多模态验收")
    parser.add_argument("--cases", type=Path, default=DEFAULT_CASES)
    parser.add_argument("--only", help="comma-separated case ids")
    parser.add_argument("--out", type=Path)
    parser.add_argument("--timeout", type=int, default=180)
    parser.add_argument("--keep-sessions", action="store_true")
    args = parser.parse_args()

    env = load_env_file(args.env_file)
    email = args.email or env.get("OWNER_EMAIL") or ""
    password = args.password or env.get("OWNER_PASSWORD") or ""
    if not email or not password:
        parser.error("OWNER_EMAIL/OWNER_PASSWORD missing; pass --email/--password or deployments/.env")

    cases = json.loads(args.cases.read_text(encoding="utf-8"))
    if args.only:
        wanted = {item.strip() for item in args.only.split(",") if item.strip()}
        cases = [case for case in cases if case["id"] in wanted]
    if not cases:
        parser.error("no cases selected")

    token = login(args.base_url, email, password)
    kb_ids = {
        "text": resolve_kb(args.base_url, token, args.text_kb),
        "image": resolve_kb(args.base_url, token, args.image_kb),
    }

    results, failed = [], 0
    for case in cases:
        session_id = create_session(args.base_url, token, f"accept-{case['id']}")
        started = time.time()
        try:
            answer, references, errors = stream_qa(
                args.base_url,
                token,
                session_id,
                {
                    "query": case["question"],
                    "knowledge_base_ids": [kb_ids[case["kb"]]],
                    "answer_mode": case["mode"],
                    "disable_title": True,
                    "channel": "api",
                },
                args.timeout,
            )
            verdict = evaluate(case, answer, references, errors)
        except Exception as exc:  # noqa: BLE001 - one failed case must not stop the rest
            verdict = {"ok": False, "problems": [str(exc)], "chars": 0, "refs": 0, "ref_files": [], "images": 0}
        finally:
            if not args.keep_sessions:
                delete_session(args.base_url, token, session_id)
        result = {"id": case["id"], "mode": case["mode"], "kb": case["kb"], "elapsed_s": round(time.time() - started, 1), **verdict}
        results.append(result)
        if not verdict["ok"]:
            failed += 1
        status = "PASS" if verdict["ok"] else "FAIL"
        detail = "" if verdict["ok"] else " — " + "; ".join(verdict["problems"])
        print(f"{status}  {case['id']} {case['mode']}/{case['kb']} chars={result['chars']} refs={result['refs']}{detail}")

    summary = {"total": len(results), "passed": len(results) - failed, "failed": failed, "results": results}
    if args.out:
        args.out.parent.mkdir(parents=True, exist_ok=True)
        args.out.write_text(json.dumps(summary, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
        print(f"report: {args.out}")
    return 1 if failed else 0


if __name__ == "__main__":
    sys.exit(main())
