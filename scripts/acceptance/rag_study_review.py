#!/usr/bin/env python3
"""Run the RAG study/review acceptance against a live WellRead stack.

Code cases validate their Mermaid ecosystem diagram with headless Chrome, so
set NODE_PATH=frontend/node_modules and CHROME_PATH before running.
"""

import argparse
import json
import os
import re
import subprocess
import sys
import tempfile
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
REQUIRED_CATEGORIES = ("concept", "compare", "process", "structure", "boundary", "code", "image")
ZERO_HIT_TERMS = ("未找到", "资料不足", "没有找到", "无法从", "不包含", "无法回答", "无法确定")
RESOURCE_URL_RE = re.compile(r"resource://[A-Za-z0-9_-]+")
FENCED_CODE_RE = re.compile(r"```.*?```", re.S)
MERMAID_BLOCK_RE = re.compile(r"```mermaid\s*(.*?)```", re.S)
MERMAID_VALIDATOR = Path(__file__).with_name("validate_mermaid.mjs")


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


def load_image_candidate_urls(base_url, token, kb_id):
    """Collect every stored image URL in the KB; used to prove that an answer
    only embeds images the retrieval candidate list actually offered."""
    urls = set()
    body = request_json(base_url, "GET", f"/api/v1/knowledge-bases/{kb_id}/knowledge?page=1&page_size=200", token=token)
    for doc in body.get("data") or []:
        doc_id = doc.get("id")
        if not doc_id:
            continue
        chunks = request_json(base_url, "GET", f"/api/v1/chunks/{doc_id}?page=1&page_size=200", token=token)
        for row in chunks.get("data") or []:
            blob = (row.get("image_info") or "") + "\n" + (row.get("content") or "")
            urls.update(RESOURCE_URL_RE.findall(blob))
    return urls


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


def _mermaid_nodes(code):
    nodes = set()
    for line in code.splitlines():
        head = re.match(r"\s*([A-Za-z_][A-Za-z0-9_]*)\s*(?:[\[(\{]|--|==)", line)
        if head:
            nodes.add(head.group(1))
        for match in re.finditer(r"-->\s*(?:\|[^|]*\|\s*)?([A-Za-z_][A-Za-z0-9_]*)", line):
            nodes.add(match.group(1))
    return len(nodes)

def check_mermaid_diagram(answer):
    """Require a renderable Mermaid flowchart with 3-8 nodes for code answers."""
    blocks = MERMAID_BLOCK_RE.findall(answer)
    if not blocks:
        return ["code answer missing mermaid ecosystem diagram"], 0
    code = blocks[0].strip()
    if not re.match(r"(flowchart|graph)\b", code):
        return ["mermaid block is not a flowchart"], 0
    nodes = _mermaid_nodes(code)
    if nodes < 3 or nodes > 8:
        return [f"mermaid diagram has {nodes} nodes, expected 3-8"], nodes
    try:
        with tempfile.NamedTemporaryFile("w", suffix=".mmd", delete=False, encoding="utf-8") as handle:
            handle.write(code)
            path = handle.name
        try:
            proc = subprocess.run(
                ["node", str(MERMAID_VALIDATOR), path],
                capture_output=True, text=True, encoding="utf-8", timeout=120, env=os.environ,
            )
        finally:
            os.unlink(path)
        line = (proc.stdout or "").strip().splitlines()[-1] if proc.stdout else ""
        result = json.loads(line) if line else {"ok": False, "error": proc.stderr[-300:]}
        if not result.get("ok"):
            return [f"mermaid parse failed: {result.get('error', 'unknown')[:200]}"], nodes
    except Exception as exc:  # noqa: BLE001 - validator missing must not crash the suite
        return [f"mermaid validator unavailable: {exc}"], nodes
    return [], nodes


def evaluate(case, answer, references, errors, candidates):
    problems = []
    if errors:
        problems.append("SSE error: " + "; ".join(errors)[:300])
    raw_chars = len(answer)
    chars = raw_chars
    if case.get("category") == "code":
        # Code answers are measured by their prose; fenced code is capped
        # separately and would otherwise dominate the character budget.
        chars = len(FENCED_CODE_RE.sub("", answer))
    if chars < case["min_chars"] or chars > case["max_chars"]:
        problems.append(f"chars {chars} outside {case['min_chars']}-{case['max_chars']}")
    if not answer:
        problems.append("empty answer")
    for term in FORBIDDEN_TERMS:
        if term in answer:
            problems.append(f"forbidden term: {term}")
    if "[[IMG:" in answer:
        problems.append("unreplaced [[IMG:n]] placeholder")
    for target in re.findall(r"!\[[^\]]*\]\(([^)]+)\)", answer):
        if not target.startswith("resource://"):
            problems.append(f"raw non-resource image markdown: {target[:80]}")

    ref_keys = []
    for ref in references:
        name = (ref.get("knowledge_filename") or ref.get("knowledge_title") or "").strip()
        ref_id = str(ref.get("id") or ref.get("app_chunk_id") or "").strip()
        if not name:
            problems.append("reference without file name")
        if not ref_id:
            problems.append("reference without traceable id")
        ref_keys.append((name, ref_id))
    if case.get("expect_zero_hit"):
        if references:
            problems.append(f"zero-hit expected no references, got {len(references)}")
        if IMAGE_RE.search(answer):
            problems.append("zero-hit answer embeds an image")
        if not any(term in answer for term in ZERO_HIT_TERMS):
            problems.append("zero-hit answer missing data-gap guidance")
        if any(term in answer for term in ("易错点", "考点清单", "自测题", "一句话结论")):
            problems.append("zero-hit answer produced a full study/review card")
    elif case.get("category") == "image":
        if not references:
            problems.append("image answer has no references")
        images = IMAGE_URL_RE.findall(answer)
        if case.get("expect_image") and not images:
            problems.append("image answer has no resource:// image")
        if candidates and images:
            unknown = [url for url in images if url not in candidates]
            if unknown:
                problems.append("image URL not in retrieval candidates: " + ", ".join(unknown[:3]))
        if len(images) != len(set(images)):
            problems.append("duplicate image URL in answer")
    elif case.get("category") == "code":
        if not references:
            problems.append("no references")
        terms = ("一句话结论", "宏观", "实现流程", "设计", "边界")
        hits = [term for term in terms if term in answer]
        if len(hits) < 2:
            problems.append(f"code template sections missing (found {hits})")
        if not re.search(r"[\w./\\-]+\.(?:go|py|js|ts|tsx|jsx|java|kt|rs|c|cc|cpp|h|hpp|cs|rb|php|swift|scala|sh|sql|lua|vue):\d+", answer):
            problems.append("code answer missing path:line location")
    else:
        if not references:
            problems.append("no references")
        terms = STUDY_TERMS if case["mode"] == "study" else REVIEW_TERMS
        hits = [term for term in terms if term in answer]
        if len(hits) < 2:
            problems.append(f"{case['mode']} sections missing (found {hits})")
        if case["mode"] == "review" and "自测" not in answer and "答案" not in answer:
            problems.append("review answer missing self-test")

    if len(ref_keys) != len(set(ref_keys)):
        problems.append("duplicate references")

    return {
        "ok": not problems,
        "problems": problems,
        "category": case.get("category", ""),
        "chars": chars,
        "raw_chars": raw_chars,
        "refs": len(references),
        "ref_files": [key[0] for key in ref_keys],
        "images": len(IMAGE_URL_RE.findall(answer)),
        "mermaid_blocks": len(MERMAID_BLOCK_RE.findall(answer)),
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
    parser.add_argument("--code-kb", default="代码导读验收")
    parser.add_argument("--cases", type=Path, default=DEFAULT_CASES)
    parser.add_argument("--only", help="comma-separated case ids")
    parser.add_argument("--out", type=Path)
    parser.add_argument("--timeout", type=int, default=180)
    parser.add_argument("--keep-sessions", action="store_true")
    parser.add_argument("--enforce-target", action="store_true",
                        help="fail when target-range adherence is below 80%")
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
    if not args.only:
        category_counts = {}
        for case in cases:
            category_counts[case.get("category", "")] = category_counts.get(case.get("category", ""), 0) + 1
        missing = [name for name in REQUIRED_CATEGORIES if category_counts.get(name, 0) < 3]
        if missing:
            parser.error(f"question bank needs at least 3 cases per category; missing: {missing}")

    token = login(args.base_url, email, password)
    kb_ids = {
        "text": resolve_kb(args.base_url, token, args.text_kb),
        "image": resolve_kb(args.base_url, token, args.image_kb),
        "code": resolve_kb(args.base_url, token, args.code_kb),
    }
    candidates = None
    if any(case.get("category") == "image" for case in cases):
        candidates = load_image_candidate_urls(args.base_url, token, kb_ids["image"])

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
            verdict = evaluate(case, answer, references, errors, candidates)
        except Exception as exc:  # noqa: BLE001 - one failed case must not stop the rest
            verdict = {"ok": False, "problems": [str(exc)], "chars": 0, "refs": 0, "ref_files": [], "images": 0}
        finally:
            if not args.keep_sessions:
                delete_session(args.base_url, token, session_id)
        target_min = case.get("target_min")
        target_max = case.get("target_max")
        within_target = None
        if target_min is not None and target_max is not None and verdict["chars"]:
            within_target = target_min <= verdict["chars"] <= target_max
        result = {
            "id": case["id"],
            "category": case.get("category", ""),
            "mode": case["mode"],
            "kb": case["kb"],
            "elapsed_s": round(time.time() - started, 1),
            "within_target": within_target,
            **verdict,
        }
        results.append(result)
        if not verdict["ok"]:
            failed += 1
        status = "PASS" if verdict["ok"] else "FAIL"
        detail = "" if verdict["ok"] else " — " + "; ".join(verdict["problems"])
        print(f"{status}  {case['id']} {case['mode']}/{case['kb']} chars={result['chars']} refs={result['refs']}{detail}")

    target_cases = [r for r in results if r.get("within_target") is not None]
    target_ok = sum(1 for r in target_cases if r["within_target"])
    target_rate = (target_ok / len(target_cases)) if target_cases else 1.0
    summary = {
        "total": len(results),
        "passed": len(results) - failed,
        "failed": failed,
        "target_cases": len(target_cases),
        "target_within": target_ok,
        "target_rate": round(target_rate, 3),
        "results": results,
    }
    if args.out:
        args.out.parent.mkdir(parents=True, exist_ok=True)
        args.out.write_text(json.dumps(summary, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
        print(f"report: {args.out}")
    print(f"target-range adherence: {target_ok}/{len(target_cases)} ({target_rate:.0%})")
    if args.enforce_target and target_rate < 0.8:
        print("target-range adherence below 80%", file=sys.stderr)
        return 1
    return 1 if failed else 0


if __name__ == "__main__":
    sys.exit(main())
