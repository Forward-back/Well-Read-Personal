#!/usr/bin/env python3
"""Reading-agent evaluation: fixtures, metrics, and old-engine baseline.

The script uploads its own fictional fixtures into a dedicated knowledge base
(never business documents), drives the reading engine through the public API,
and reports three P0 metrics:

- quote hit rate: persisted evidence whose quote is verbatim inside its chunk
- key-point recall: human-labelled key points found in conclusions, terms,
  contradictions, or the run summary
- self-test valid rate: only when the engine exposes self-tests (v3); the old
  engine reports N/A

Usage:
    python scripts/acceptance/reading_agent_metrics.py \
        --out scripts/acceptance/reading_agent_baseline.json

Owner credentials come from deployments/.env (OWNER_EMAIL/OWNER_PASSWORD).
"""

import argparse
import json
import sys
import time
import unicodedata
import urllib.error
import urllib.parse
import urllib.request
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
FIXTURES = Path(__file__).with_name("fixtures") / "reading_agent"
DEFAULT_CASES = Path(__file__).with_name("reading_agent_cases.json")
TERMINAL = {"finished", "stopped", "abandoned", "failed"}


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


class HTTPFailure(RuntimeError):
    def __init__(self, method, path, status, body):
        super().__init__(f"{method} {path} -> HTTP {status}: {body[:500]}")
        self.status = status


def request_json(base_url, method, path, token=None, payload=None, timeout=600):
    data = None
    headers = {"Accept": "application/json"}
    if payload is not None:
        data = json.dumps(payload, ensure_ascii=False).encode("utf-8")
        headers["Content-Type"] = "application/json"
    if token:
        headers["Authorization"] = "Bearer " + token
    req = urllib.request.Request(
        base_url.rstrip("/") + path, data=data, headers=headers, method=method
    )
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            raw = resp.read().decode("utf-8", "replace")
    except urllib.error.HTTPError as exc:
        raise HTTPFailure(method, path, exc.code, exc.read().decode("utf-8", "replace")) from exc
    if not raw:
        return {}
    try:
        return json.loads(raw)
    except json.JSONDecodeError as exc:
        raise RuntimeError(f"{method} {path} returned non-JSON: {raw[:300]}") from exc


def unwrap(body):
    if isinstance(body, dict) and "data" in body:
        return body["data"]
    return body


def login(base_url, email, password):
    body = request_json(
        base_url, "POST", "/api/v1/auth/login",
        payload={"email": email, "password": password}, timeout=60,
    )
    token = body.get("token") or (body.get("data") or {}).get("token")
    if not token:
        raise RuntimeError("login response contains no token")
    return token


def list_knowledge_bases(base_url, token):
    rows = unwrap(request_json(base_url, "GET", "/api/v1/knowledge-bases", token=token))
    return rows if isinstance(rows, list) else []


def resolve_embedding_model(base_url, token, kbs, wanted):
    if wanted:
        return wanted
    for kb in kbs:
        model_id = (kb.get("embedding_model_id") or "").strip()
        if model_id:
            return model_id
    rows = unwrap(request_json(base_url, "GET", "/api/v1/models?page=1&page_size=100", token=token))
    if isinstance(rows, dict):
        rows = rows.get("list") or rows.get("models") or []
    for model in rows if isinstance(rows, list) else []:
        if (model.get("type") or "").lower() == "embedding":
            return model.get("id")
    raise RuntimeError("no embedding model available; pass --embedding-model")


def ensure_kb(base_url, token, name, embedding_model):
    for kb in list_knowledge_bases(base_url, token):
        if (kb.get("name") or "").strip() == name:
            return kb.get("id"), False
    payload = {
        "name": name,
        "type": "document",
        "embedding_model_id": embedding_model,
    }
    body = request_json(base_url, "POST", "/api/v1/knowledge-bases", token=token, payload=payload)
    kb_id = (unwrap(body) or {}).get("id")
    if not kb_id:
        raise RuntimeError(f"knowledge base creation returned no id: {body}")
    return kb_id, True


def list_knowledge(base_url, token, kb_id):
    rows = unwrap(
        request_json(
            base_url, "GET",
            f"/api/v1/knowledge-bases/{kb_id}/knowledge?page=1&page_size=200",
            token=token,
        )
    )
    return rows if isinstance(rows, list) else []


def knowledge_status(base_url, token, kb_id, doc_id):
    body = unwrap(request_json(base_url, "GET", f"/api/v1/knowledge/{doc_id}", token=token))
    return (body or {}).get("parse_status") or "", body or {}


def wait_parse(base_url, token, kb_id, doc_id, timeout=600):
    started = time.time()
    last = ""
    while time.time() - started < timeout:
        status, body = knowledge_status(base_url, token, kb_id, doc_id)
        last = status
        if status == "completed":
            return body
        if status in ("failed", "parse_failed"):
            raise RuntimeError(f"knowledge {doc_id} parse failed")
        time.sleep(3)
    raise RuntimeError(f"knowledge {doc_id} parse timeout, last status={last}")


def delete_knowledge(base_url, token, doc_id):
    try:
        request_json(base_url, "DELETE", f"/api/v1/knowledge/{doc_id}", token=token, timeout=120)
    except Exception as exc:  # noqa: BLE001 - reset is best effort
        print(f"warn: delete knowledge {doc_id} failed: {exc}", file=sys.stderr)


def ensure_fixture_docs(base_url, token, kb_id, cases, reset=False):
    existing = {((doc.get("title") or "").strip()): doc for doc in list_knowledge(base_url, token, kb_id)}
    doc_ids = {}
    for case in cases:
        for doc in case["documents"]:
            title = doc["title"]
            path = FIXTURES / doc["file"]
            content = path.read_text(encoding="utf-8")
            current = existing.get(title)
            if current and reset:
                delete_knowledge(base_url, token, current.get("id"))
                current = None
            if current and current.get("parse_status") == "completed":
                doc_ids[title] = current.get("id")
                continue
            if current and current.get("parse_status") not in ("completed", "failed"):
                wait_parse(base_url, token, kb_id, current.get("id"))
                doc_ids[title] = current.get("id")
                continue
            body = request_json(
                base_url, "POST",
                f"/api/v1/knowledge-bases/{kb_id}/knowledge/manual",
                token=token,
                payload={"title": title, "content": content, "status": "publish"},
                timeout=120,
            )
            created = unwrap(body) or {}
            doc_id = created.get("id")
            if not doc_id:
                raise RuntimeError(f"manual knowledge creation returned no id: {body}")
            wait_parse(base_url, token, kb_id, doc_id)
            doc_ids[title] = doc_id
            print(f"  fixture uploaded: {title} ({doc_id})")
    return doc_ids


def list_chunks(base_url, token, doc_id):
    chunks = []
    page = 1
    while True:
        rows = unwrap(
            request_json(
                base_url, "GET",
                f"/api/v1/chunks/{doc_id}?page={page}&page_size=200",
                token=token,
            )
        )
        if not isinstance(rows, list) or not rows:
            break
        chunks.extend(rows)
        if len(rows) < 200:
            break
        page += 1
    return chunks


def create_session(base_url, token, title):
    body = request_json(base_url, "POST", "/api/v1/sessions", token=token, payload={"title": title})
    session_id = (unwrap(body) or {}).get("id")
    if not session_id:
        raise RuntimeError("session creation returned no id")
    return session_id


def delete_session(base_url, token, session_id):
    try:
        request_json(base_url, "DELETE", f"/api/v1/sessions/{session_id}", token=token, timeout=60)
    except Exception as exc:  # noqa: BLE001 - cleanup must not mask results
        print(f"warn: delete session {session_id} failed: {exc}", file=sys.stderr)


def get_run(base_url, token, run_id):
    return unwrap(request_json(base_url, "GET", f"/api/v1/reading/runs/{run_id}", token=token))


def drive_run(base_url, token, run_id, case_timeout, max_steps):
    started = time.time()
    step_calls = 0
    step_errors = []
    resumes = 0
    last = get_run(base_url, token, run_id)
    while time.time() - started < case_timeout and step_calls < max_steps:
        status = last.get("status")
        if status == "finished":
            return last, step_calls, step_errors, round(time.time() - started, 1)
        if status == "waiting_user":
            pending = [q for q in (last.get("questions") or []) if q.get("status") == "pending"]
            if not pending:
                time.sleep(1)
                last = get_run(base_url, token, run_id)
                continue
            for question in pending:
                request_json(
                    base_url, "POST", f"/api/v1/reading/runs/{run_id}/feedback",
                    token=token, payload={"skip": True, "question_id": question.get("id")}, timeout=120,
                )
            last = get_run(base_url, token, run_id)
            continue
        if status in ("stopped", "abandoned"):
            return last, step_calls, step_errors, round(time.time() - started, 1)
        if status == "failed":
            if resumes >= 2:
                return last, step_calls, step_errors, round(time.time() - started, 1)
            resumes += 1
            try:
                request_json(base_url, "POST", f"/api/v1/reading/runs/{run_id}/resume", token=token, timeout=120)
            except HTTPFailure as exc:
                step_errors.append(str(exc))
                return last, step_calls, step_errors, round(time.time() - started, 1)
            last = get_run(base_url, token, run_id)
            continue
        try:
            step_calls += 1
            last = unwrap(
                request_json(
                    base_url, "POST", f"/api/v1/reading/runs/{run_id}/step",
                    token=token, timeout=900,
                )
            ) or get_run(base_url, token, run_id)
        except HTTPFailure as exc:
            step_errors.append(str(exc))
            last = get_run(base_url, token, run_id)
            if last.get("status") in ("stopped", "failed") and resumes < 2:
                resumes += 1
                try:
                    request_json(
                        base_url, "POST", f"/api/v1/reading/runs/{run_id}/resume",
                        token=token, timeout=120,
                    )
                    last = get_run(base_url, token, run_id)
                    continue
                except HTTPFailure as resume_exc:
                    step_errors.append(str(resume_exc))
                    return last, step_calls, step_errors, round(time.time() - started, 1)
            if last.get("status") in TERMINAL:
                return last, step_calls, step_errors, round(time.time() - started, 1)
            time.sleep(2)
    return last, step_calls, step_errors, round(time.time() - started, 1)


def fetch_self_tests(base_url, token, run_id):
    try:
        body = request_json(
            base_url, "GET", f"/api/v1/reading/runs/{run_id}/self-tests", token=token, timeout=120,
        )
    except HTTPFailure as exc:
        if exc.status in (404, 405):
            return None
        raise
    data = unwrap(body)
    if isinstance(data, dict):
        for key in ("self_tests", "tests", "items", "data"):
            if isinstance(data.get(key), list):
                return data[key]
        return []
    return data if isinstance(data, list) else []


def flatten_review_groups(nodes):
    out = []
    for node in nodes or []:
        group = node.get("group") or {}
        if group.get("id"):
            out.append(group)
        out.extend(flatten_review_groups(node.get("children") or []))
    return out


def verify_self_test_import(base_url, token, results, cleanup=True):
    """Import one candidate into the default review group, verify the status
    transition, then delete the created card so evaluation leaves no sample."""
    target = None
    for result in results:
        if not result.get("run_id"):
            continue
        tests = fetch_self_tests(base_url, token, result["run_id"])
        candidate = next(
            (item for item in (tests or [])
             if item.get("status") == "candidate" and item.get("question")),
            None,
        )
        if candidate:
            target = (result["run_id"], candidate)
            break
    if not target:
        return {"ok": False, "reason": "no candidate self-test to import"}
    run_id, candidate = target
    groups = flatten_review_groups(unwrap(request_json(base_url, "GET", "/api/v1/review/groups", token=token)))
    group = next((item for item in groups if item.get("is_default")), groups[0] if groups else None)
    if not group:
        return {"ok": False, "reason": "no review group available"}
    import_body = unwrap(request_json(
        base_url, "POST", f"/api/v1/reading/runs/{run_id}/self-tests/import",
        token=token,
        payload={"group_id": group["id"], "self_test_ids": [candidate["id"]]},
        timeout=120,
    ))
    imported = (import_body or {}).get("imported") or 0
    refreshed = fetch_self_tests(base_url, token, run_id) or []
    stored = next((item for item in refreshed if item.get("id") == candidate["id"]), {})
    ok = imported == 1 and stored.get("status") == "imported"
    created_cards = []
    if cleanup:
        cards = unwrap(request_json(
            base_url, "GET",
            f"/api/v1/review/cards?group_id={group['id']}&keyword={urllib.parse.quote(candidate['question'][:60])}",
            token=token,
        )) or []
        for card in cards if isinstance(cards, list) else []:
            if (card.get("question") or "").strip() == candidate["question"].strip():
                created_cards.append(card.get("id"))
                request_json(base_url, "DELETE", f"/api/v1/review/cards/{card.get('id')}", token=token)
    return {
        "ok": ok,
        "run_id": run_id,
        "self_test_id": candidate["id"],
        "group_id": group["id"],
        "imported": imported,
        "stored_status": stored.get("status"),
        "cleaned_cards": len(created_cards),
    }


def normalize_text(value):
    out = []
    for char in value or "":
        category = unicodedata.category(char)
        if char.isspace() or category.startswith("P") or category.startswith("S"):
            continue
        out.append(char.lower())
    return "".join(out)


def iter_evidence(run, source):
    if source == "questions":
        items = run.get("questions") or []
        for item in items:
            for evidence in item.get("evidence") or []:
                yield evidence
        return
    if source == "terms":
        items = run.get("terms") or []
        for item in items:
            if item.get("status") == "rejected":
                continue
            for evidence in item.get("source") or []:
                yield evidence
        return
    if source == "contradictions":
        items = run.get("contradictions") or []
        for item in items:
            for evidence in (item.get("evidence_a") or []) + (item.get("evidence_b") or []):
                yield evidence
        return
    items = run.get("conclusions") or []
    for item in items:
        if item.get("status") == "rejected":
            continue
        for evidence in item.get("evidence") or []:
            yield evidence


def evidence_metrics(run, chunk_texts):
    total = 0
    valid = 0
    problems = []
    sources = {}
    for source in ("conclusions", "terms", "contradictions", "questions"):
        source_total = 0
        source_valid = 0
        for evidence in iter_evidence(run, source):
            source_total += 1
            quote = evidence.get("quote") or ""
            chunk_id = evidence.get("chunk_id") or ""
            content = chunk_texts.get(chunk_id)
            normalized_quote = normalize_text(quote)
            ok = bool(content) and len(normalized_quote) >= 4 and normalized_quote in normalize_text(content)
            if ok:
                source_valid += 1
            elif len(problems) < 20:
                problems.append({
                    "source": source,
                    "chunk_id": chunk_id,
                    "quote": quote[:120],
                    "reason": "unknown chunk" if not content else "quote not verbatim",
                })
        total += source_total
        valid += source_valid
        sources[source] = {"total": source_total, "valid": source_valid}
    rate = (valid / total) if total else None
    return {
        "total": total,
        "valid": valid,
        "hit_rate": round(rate, 4) if rate is not None else None,
        "by_source": sources,
        "problems": problems,
    }


def finding_texts(run):
    findings = []

    def add(source, text, evidence):
        combined = text or ""
        for item in evidence or []:
            combined += "\n" + (item.get("quote") or "")
        findings.append({"source": source, "text": combined})

    for item in run.get("conclusions") or []:
        if item.get("status") != "rejected":
            add("conclusion", item.get("text"), item.get("evidence"))
    for item in run.get("terms") or []:
        if item.get("status") != "rejected":
            add("term", f"{item.get('term') or ''}：{item.get('definition') or ''}", item.get("source"))
    for item in run.get("contradictions") or []:
        add(
            "contradiction",
            item.get("description"),
            (item.get("evidence_a") or []) + (item.get("evidence_b") or []),
        )
    final = run.get("final")
    if isinstance(final, dict) and final.get("summary"):
        add("summary", str(final.get("summary")), [])
    return findings


def key_point_metrics(case, run):
    findings = finding_texts(run)
    results = []
    for point in case.get("key_points") or []:
        matched_source = ""
        matched_finding = ""
        for finding in findings:
            text = finding["text"].lower()
            for group in point.get("match_groups") or []:
                if all(str(token).lower() in text for token in group):
                    matched_source = finding["source"]
                    matched_finding = finding["text"][:160]
                    break
            if matched_source:
                break
        results.append({
            "id": point.get("id"),
            "statement": point.get("statement"),
            "matched": bool(matched_source),
            "matched_source": matched_source,
            "matched_finding": matched_finding,
        })
    matched = sum(1 for item in results if item["matched"])
    total = len(results)
    return {
        "total": total,
        "matched": matched,
        "recall": round(matched / total, 4) if total else None,
        "points": results,
    }


def self_test_metrics(self_tests, chunk_texts):
    if self_tests is None:
        return {"generated": None, "valid": None, "valid_rate": None, "note": "engine has no self-tests"}
    seen = set()
    unique = 0
    valid = 0
    for item in self_tests:
        question = normalize_text(item.get("question") or "")
        if question and question not in seen:
            seen.add(question)
            unique += 1
        answer = (item.get("answer") or "").strip()
        evidence = item.get("evidence") or []
        evidence_ok = False
        for ref in evidence:
            quote = normalize_text(ref.get("quote") or "")
            content = normalize_text(chunk_texts.get(ref.get("chunk_id") or "") or "")
            if len(quote) >= 4 and content and quote in content:
                evidence_ok = True
                break
        if question and answer and evidence_ok:
            valid += 1
    total = len(self_tests)
    return {
        "generated": total,
        "unique": unique,
        "valid": valid,
        "valid_rate": round(valid / total, 4) if total else None,
    }


def degraded_rounds(run):
    count = 0
    for step in run.get("steps") or []:
        patch = step.get("notes_patch")
        if isinstance(patch, dict) and patch.get("degraded"):
            count += 1
        elif isinstance(patch, str) and '"degraded"' in patch and '"degraded":true' in patch.replace(" ", ""):
            count += 1
    return count


def resolve_run_docs(case, doc_ids):
    selected = []
    for doc in case["documents"]:
        doc_id = doc_ids.get(doc["title"])
        if doc_id:
            selected.append(doc_id)
    return selected


def run_case(base_url, token, case, doc_ids, args):
    run_started = time.time()
    session_id = create_session(base_url, token, f"reading-agent-eval-{case['id']}-{int(run_started)}")
    document_ids = resolve_run_docs(case, doc_ids)
    if not document_ids:
        raise RuntimeError("case has no resolved documents")
    body = request_json(
        base_url, "POST", "/api/v1/reading/runs", token=token,
        payload={
            "session_id": session_id,
            "document_ids": document_ids,
            "depth": case.get("depth") or args.depth,
            "goal": case.get("goal") or "",
        },
        timeout=120,
    )
    run = unwrap(body) or {}
    run_id = run.get("id")
    if not run_id:
        raise RuntimeError("reading run creation returned no id")
    try:
        final_run, step_calls, step_errors, elapsed = drive_run(
            base_url, token, run_id, args.case_timeout, args.max_steps,
        )
        chunk_texts = {}
        for doc_id in document_ids:
            for chunk in list_chunks(base_url, token, doc_id):
                if chunk.get("id"):
                    chunk_texts[chunk["id"]] = chunk.get("content") or ""
        self_tests = fetch_self_tests(base_url, token, run_id)
        plan = final_run.get("plan") or {}
        metrics = {
            "status": final_run.get("status"),
            "state_version": plan.get("state_version"),
            "step_count": final_run.get("step_count"),
            "token_usage": final_run.get("token_usage"),
            "elapsed_s": elapsed,
            "step_calls": step_calls,
            "step_errors": step_errors,
            "degraded_rounds": degraded_rounds(final_run),
            "counts": {
                "conclusions": len(final_run.get("conclusions") or []),
                "terms": len(final_run.get("terms") or []),
                "contradictions": len(final_run.get("contradictions") or []),
                "questions": len(final_run.get("questions") or []),
            },
            "evidence": evidence_metrics(final_run, chunk_texts),
            "key_points": key_point_metrics(case, final_run),
            "self_tests": self_test_metrics(self_tests, chunk_texts),
        }
        return {
            "id": case["id"],
            "title": case.get("title"),
            "run_id": run_id,
            "ok": final_run.get("status") == "finished" and not step_errors,
            "metrics": metrics,
        }
    finally:
        if not args.keep_sessions:
            delete_session(base_url, token, session_id)


def main():
    try:
        sys.stdout.reconfigure(encoding="utf-8")
    except Exception:  # noqa: BLE001 - older consoles
        pass
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--env-file", type=Path, default=ROOT / "deployments" / ".env")
    parser.add_argument("--base-url", default="http://localhost:6152")
    parser.add_argument("--email")
    parser.add_argument("--password")
    parser.add_argument("--cases", type=Path, default=DEFAULT_CASES)
    parser.add_argument("--kb", default="精读agent评测集")
    parser.add_argument("--embedding-model", default="")
    parser.add_argument("--depth", default="standard")
    parser.add_argument("--only", help="comma-separated case ids")
    parser.add_argument("--engine", default="old", help="label for the engine under test")
    parser.add_argument("--out", type=Path)
    parser.add_argument("--reset-docs", action="store_true", help="delete and recreate evaluation fixtures")
    parser.add_argument("--keep-sessions", action="store_true")
    parser.add_argument("--verify-import", action="store_true",
                        help="import one self-test into the default review group and clean it up")
    parser.add_argument("--keep-imported", action="store_true")
    parser.add_argument("--case-timeout", type=int, default=1800)
    parser.add_argument("--max-steps", type=int, default=200)
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
    for case in cases:
        for doc in case.get("documents") or []:
            if not (FIXTURES / doc["file"]).exists():
                parser.error(f"fixture missing: {FIXTURES / doc['file']}")

    token = login(args.base_url, email, password)
    kbs = list_knowledge_bases(args.base_url, token)
    embedding_model = resolve_embedding_model(args.base_url, token, kbs, args.embedding_model)
    kb_id, created = ensure_kb(args.base_url, token, args.kb, embedding_model)
    print(f"eval KB: {args.kb} ({kb_id}){' created' if created else ''}")
    doc_ids = ensure_fixture_docs(args.base_url, token, kb_id, cases, reset=args.reset_docs)

    results = []
    for case in cases:
        print(f"CASE {case['id']} …")
        started = time.time()
        try:
            result = run_case(args.base_url, token, case, doc_ids, args)
        except Exception as exc:  # noqa: BLE001 - one failed case must not stop the rest
            result = {
                "id": case["id"],
                "title": case.get("title"),
                "ok": False,
                "error": str(exc),
                "metrics": {"elapsed_s": round(time.time() - started, 1)},
            }
        results.append(result)
        metrics = result.get("metrics") or {}
        evidence = metrics.get("evidence") or {}
        recall = (metrics.get("key_points") or {}).get("recall")
        status = "PASS" if result.get("ok") else "FAIL"
        detail = metrics.get("status") or result.get("error", "")
        print(
            f"{status}  {case['id']} status={detail} "
            f"quote_hit={evidence.get('hit_rate')} recall={recall} "
            f"steps={metrics.get('step_count')} tokens={metrics.get('token_usage')} "
            f"elapsed={metrics.get('elapsed_s')}s"
        )
        if result.get("error"):
            print(f"  error: {result['error']}", file=sys.stderr)

    evidence_total = sum((r.get("metrics", {}).get("evidence") or {}).get("total", 0) for r in results)
    evidence_valid = sum((r.get("metrics", {}).get("evidence") or {}).get("valid", 0) for r in results)
    points_total = sum((r.get("metrics", {}).get("key_points") or {}).get("total", 0) for r in results)
    points_matched = sum((r.get("metrics", {}).get("key_points") or {}).get("matched", 0) for r in results)
    self_tests_present = any(
        (r.get("metrics", {}).get("self_tests") or {}).get("generated") is not None for r in results
    )
    self_generated = sum(
        ((r.get("metrics", {}).get("self_tests") or {}).get("generated") or 0) for r in results
    )
    self_valid = sum(((r.get("metrics", {}).get("self_tests") or {}).get("valid") or 0) for r in results)
    summary = {
        "engine": args.engine,
        "generated_at": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
        "base_url": args.base_url,
        "kb": args.kb,
        "cases": len(results),
        "passed": sum(1 for r in results if r.get("ok")),
        "quote_hit_rate": round(evidence_valid / evidence_total, 4) if evidence_total else None,
        "evidence_total": evidence_total,
        "evidence_valid": evidence_valid,
        "key_points_total": points_total,
        "key_points_matched": points_matched,
        "key_point_recall": round(points_matched / points_total, 4) if points_total else None,
        "self_tests_generated": self_generated if self_tests_present else None,
        "self_tests_valid": self_valid if self_tests_present else None,
        "self_test_valid_rate": (
            round(self_valid / self_generated, 4) if self_tests_present and self_generated else None
        ),
        "total_tokens": sum((r.get("metrics", {}).get("token_usage") or 0) for r in results),
        "total_elapsed_s": round(sum((r.get("metrics", {}).get("elapsed_s") or 0) for r in results), 1),
        "degraded_rounds": sum((r.get("metrics", {}).get("degraded_rounds") or 0) for r in results),
    }
    output = {"summary": summary, "results": results}
    if args.verify_import:
        try:
            import_evidence = verify_self_test_import(
                args.base_url, token, results, cleanup=not args.keep_imported,
            )
        except Exception as exc:  # noqa: BLE001 - evidence failure must not hide metrics
            import_evidence = {"ok": False, "reason": str(exc)}
        output["self_test_import"] = import_evidence
        print("SELF-TEST IMPORT " + json.dumps(import_evidence, ensure_ascii=False))
    print("\nSUMMARY " + json.dumps(summary, ensure_ascii=False))
    if args.out:
        args.out.parent.mkdir(parents=True, exist_ok=True)
        args.out.write_text(json.dumps(output, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
        print(f"report: {args.out}")
    failed = summary["passed"] != summary["cases"]
    return 1 if failed else 0


if __name__ == "__main__":
    sys.exit(main())
