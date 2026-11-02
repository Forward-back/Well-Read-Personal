# -*- coding: utf-8 -*-
"""本地融合链路 e2e 冒烟测试(对 5051 端口 YLSW 实例)。

覆盖:markdown ingest(§2.14 元数据)、xlsx ingest(Text2SQL 路)、
/documents/ids、/search(kb+密级过滤)、/chat/stream(生命周期标签)。
"""
import base64
import json
import os
import sys

import requests

BASE = "http://127.0.0.1:5051"
HERE = os.path.dirname(os.path.abspath(__file__))

PASS, FAIL = [], []


def check(name, cond, detail=""):
    (PASS if cond else FAIL).append(name)
    print(f"[{'PASS' if cond else 'FAIL'}] {name} {detail}")


MD_DOC = {
    "knowledge_base_id": "kb-e2e-a",
    "document_id": "doc-e2e-md-1",
    "file_name": "REG_20250101_银联商务_测试产品_商户结算管理办法_V1.0_ZH.md",
    "file_type": "md",
    "markdown": (
        "# 商户结算管理办法\n\n## 第一条 目的\n\n为规范商户结算行为，制定本办法。\n\n"
        "## 第二条 结算周期\n\n商户结算周期为 T+1 工作日，每日 16 点前完成清算。\n\n"
        "## 第三条 废止说明\n\n本办法自 2026-07-01 起被《商户结算管理办法（2026 版）》替代。\n"
    ),
    "doc_number": "测试2025001号",
    "org_name": "银联商务",
    "doc_date": "2025-01-01",
    "doc_title": "商户结算管理办法",
    "doc_type": "regulation",
    "product_name": "测试产品",
    "lifecycle_status": "expired",
    "effective_date": "2025-01-01",
    "expiry_date": "2026-07-01",
    "superseded_by": "测试2026001号",
    "custom_metadata": {"适用区域": "全国"},
}


def main():
    # 1. markdown ingest(含 §2.14 元数据)
    r = requests.post(f"{BASE}/ingest", json=MD_DOC, timeout=120)
    check("md ingest 200", r.status_code == 200, r.text[:120])
    body = r.json()
    check("md ingest success+embedded>0", body.get("status") == "success" and body.get("embedded", 0) > 0,
          f"chunks={body.get('chunks')} embedded={body.get('embedded')}")

    # 2. xlsx ingest(Text2SQL 路,file_bytes)
    xlsx_path = os.path.join(HERE, "..", "Data",
                             "TBL_20260202_通用_通用_基础信息统计表 站所路段运营公司_V1.0_ZH.xlsx")
    with open(xlsx_path, "rb") as f:
        xlsx_bytes = f.read()
    r = requests.post(f"{BASE}/ingest", json={
        "knowledge_base_id": "kb-e2e-a",
        "document_id": "doc-e2e-xlsx-1",
        "file_name": os.path.basename(xlsx_path),
        "file_type": "xlsx",
        "markdown": "",
        "file_bytes": base64.b64encode(xlsx_bytes).decode(),
        "doc_number": "TBL2026001", "org_name": "银联商务", "doc_title": "基础信息统计表",
        "doc_type": "table", "product_name": "通用",
    }, timeout=120)
    check("xlsx ingest 200", r.status_code == 200, r.text[:150])
    body = r.json()
    check("xlsx ingest success+tables>0", body.get("status") == "success" and body.get("tables", 0) > 0,
          f"tables={body.get('tables')}")

    # 3. /documents/ids
    r = requests.get(f"{BASE}/documents/ids", timeout=30)
    ids = r.json().get("ids", [])
    check("/documents/ids 含两个 doc", set(ids) >= {"doc-e2e-md-1", "doc-e2e-xlsx-1"}, str(ids))

    # 4. /search(kb 白名单)
    r = requests.post(f"{BASE}/search", json={
        "queries": ["商户结算周期"],
        "knowledge_base_ids": ["kb-e2e-a"],
        "top_k": 5,
    }, timeout=120)
    results = r.json().get("results", [])
    check("/search kb 过滤有结果", len(results) > 0, f"results={len(results)}")
    check("/search 结果来自测试文档", any("商户结算" in (x.get("file_name") or "") for x in results),
          str([x.get("file_name") for x in results][:3]))

    # 5. /chat/stream(生命周期:expired 文档应带已废止标签)
    events = []
    with requests.post(f"{BASE}/chat/stream", json={
        "question": "商户结算周期是多少",
        "knowledge_base_ids": ["kb-e2e-a"],
    }, stream=True, timeout=120) as resp:
        for line in resp.iter_lines():
            if not line:
                continue
            line = line.decode("utf-8")
            if line.startswith("data: "):
                events.append(json.loads(line[6:]))
    refs = next((e.get("references") for e in events if e.get("type") == "references"), [])
    deltas = "".join(e.get("content", "") for e in events if e.get("type") == "delta")
    check("chat/stream references 带 lifecycle_status", any(x.get("lifecycle_status") == "expired" for x in refs),
          str([x.get("lifecycle_status") for x in refs]))
    check("chat/stream 产出答案", len(deltas) > 0, deltas[:80])
    check("chat/stream done 事件", any(e.get("type") == "done" for e in events))


