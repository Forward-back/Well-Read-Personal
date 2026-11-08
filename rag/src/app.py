"""
Flask HTTP API 主服务 —— 暴露 POST /query 接口。
"""

import os, sys, re, atexit, logging, threading, time, json, queue, shutil, hashlib
from difflib import SequenceMatcher
from concurrent.futures import ThreadPoolExecutor

_src = os.path.dirname(os.path.abspath(__file__))
if _src not in sys.path:
    sys.path.insert(0, _src)

from utils import setup_windows_console
setup_windows_console()

from config import (
    MYSQL_CONFIG, FINAL_TOP_K, FAISS_TOP_K, BM25_TOP_K, LOG_LEVEL,
    RERANK_SKIP_SINGLE, EMBEDDING_DIM,
    BGE_EMBED_BACKEND, BGE_RERANK_BACKEND, ENGINE_TOKEN,
    RRF_K, RERANK_META_MIN_SCORE, DEEPSEEK_MODEL,
    ANSWER_MODE_BUDGETS, DEFAULT_ANSWER_MODE, ANSWER_MIN_RERANK_SCORE,
    validate as config_validate,
)
import pymysql

logging.basicConfig(
    level=getattr(logging, LOG_LEVEL.upper(), logging.INFO),
    format="%(asctime)s [%(levelname)s] [req:%(trace)s] %(message)s",
)
logger = logging.getLogger("api")

from trace_logging import install_trace_filter, start_trace, clear_trace, submit_with_trace
install_trace_filter()

from flask import Flask, request, jsonify, Response, stream_with_context

from retrieval.faiss_search import FAISSSearcher, get_bge_embeddings
from retrieval.bm25_search import BM25Searcher
from retrieval.chunk_store import ChunkStore
from retrieval.fusion import FusionEngine
from retrieval.planner import plan_query, execute_plan, generate_query_variants
from llm.generator import (
    generate_answer, generate_synthesis_answer, generate_chitchat_answer,
)
from prompt_guard import REFUSAL_ANSWER

app = Flask(__name__)
# 竞赛评测按 UTF-8 文本读响应体，禁用 jsonify 默认的 ASCII 转义（\uXXXX）
app.json.ensure_ascii = False
app.json.mimetype = "application/json; charset=utf-8"

# 允许测试页以 file:// 方式直接打开时跨域调用 API
@app.after_request
def _add_cors_headers(resp):
    resp.headers["Access-Control-Allow-Origin"] = "*"
    resp.headers["Access-Control-Allow-Headers"] = "Content-Type, X-Engine-Token"
    resp.headers["Access-Control-Allow-Methods"] = "POST, GET, OPTIONS"
    return resp

# 服务间鉴权(§2.13 P2): 配置 YLSW_ENGINE_TOKEN 后强制 X-Engine-Token 头;
# 未配置时放行(竞赛评测/离线开发兼容)。/health 永远豁免。
@app.before_request
def _check_engine_token():
    if request.path == "/health" or not ENGINE_TOKEN:
        return None
    token = request.headers.get("X-Engine-Token", "").strip()
    if token != ENGINE_TOKEN:
        logger.warning(f"[鉴权] 请求被拒(path={request.path}, token={'有' if token else '无'})")
        return jsonify({"error": "invalid or missing X-Engine-Token"}), 401
    return None

# ── 延迟初始化 ──
faiss_searcher: FAISSSearcher | None = None
bm25_searcher: BM25Searcher | None = None
fusion: FusionEngine | None = None
# 连接池（DBUtils）：替代原版单连接 + 全局锁的串行访问
db_pool = None
chunk_store: ChunkStore | None = None


def init_services():
    global faiss_searcher, bm25_searcher, fusion, db_pool, chunk_store
    config_validate()
    logger.info("正在初始化检索服务...")
    from dbutils.pooled_db import PooledDB
    db_pool = PooledDB(
        creator=pymysql, maxconnections=8, mincached=2, maxcached=4,
        blocking=True, autocommit=True, **MYSQL_CONFIG,
    )
    _ensure_image_columns(db_pool)
    faiss_searcher = FAISSSearcher()
    fusion = FusionEngine()
    chunk_store = ChunkStore(db_pool)
    atexit.register(_cleanup)

    # BM25 词法路：启动时从池借连接构建内存索引，失败仅降级关闭该路（不阻断启动）
    try:
        bm25_searcher = BM25Searcher(db_pool.connection())
    except Exception as e:
        logger.error(f"BM25 索引初始化异常，词法路降级关闭: {e}")
        bm25_searcher = None

    # 本地模型预热（必须在 app 进程内加载：entrypoint 的预热是独立进程，退出即失效；
    # 不预热则首问触发懒加载 ~4.5s，实测首问检索三段 4493ms）
    if BGE_EMBED_BACKEND == "local" or BGE_RERANK_BACKEND == "local":
        try:
            from models_singleton import encode_texts, rerank_scores
            if BGE_EMBED_BACKEND == "local":
                encode_texts(["预热"], EMBEDDING_DIM)
                logger.info("本地 embedding 模型已预热")
            if BGE_RERANK_BACKEND == "local":
                rerank_scores("预热", ["预热"])
                logger.info("本地 reranker 模型已预热")
        except Exception as e:
            logger.warning(f"本地模型预热失败（首问会慢）: {e}")

    logger.info("所有服务初始化完成 ✓")


def _ensure_image_columns(pool):
    """存量库增量迁移：补齐图片块字段，避免旧库启动即失败。"""
    columns = {
        "chunk_type": "VARCHAR(20) DEFAULT 'text'",
        "image_url": "TEXT",
        "image_caption": "TEXT",
        "image_ocr": "TEXT",
        "parent_source_chunk_id": "VARCHAR(64)",
    }
    conn = pool.connection()
    try:
        with conn.cursor() as cur:
            cur.execute("SHOW COLUMNS FROM chunks")
            existing = {row[0] for row in cur.fetchall()}
            for name, ddl in columns.items():
                if name not in existing:
                    cur.execute(f"ALTER TABLE chunks ADD COLUMN {name} {ddl}")
                    logger.info(f"[schema] chunks.{name} 已补齐")
            cur.execute("SHOW INDEX FROM chunks WHERE Key_name='idx_image_parent'")
            if not cur.fetchall():
                cur.execute("ALTER TABLE chunks ADD INDEX idx_image_parent (parent_source_chunk_id)")
                logger.info("[schema] chunks.idx_image_parent 已补齐")
    finally:
        conn.close()


def _cleanup():
    if db_pool:
        db_pool.close()


def _run_query(question: str, collect_timing: bool = False, collect_trace: bool = False,
               collect_refs: bool = False, on_refs=None, on_delta=None,
               on_images=None, on_stage=None,
               kb_ids: list | None = None,
               skill: dict | None = None,
               skill_skipped: bool = False,
               include_expired: bool = False,
               answer_mode: str = DEFAULT_ANSWER_MODE):
    """主查询管线：规划器 → 并行检索 → RRF → 精排 → Parent → LLM，返回答案文本。

    collect_timing=True 时返回 (answer, timings, trace, refs)，timings 为各阶段耗时字典（毫秒）。
    collect_trace=True 时 trace 为检索链中间数据（评测用）。
    collect_refs=True 时 refs 为精排后的引用列表（app fusion SSE 用）。
    on_refs(refs)：精排完成后回调（SSE references 事件）。
    on_delta(text)：LLM 增量回调（SSE delta 事件）。
    on_stage(name, counts)：检索阶段回调（SSE stage 事件，前端状态栏如实展示）。
    kb_ids：KB 白名单（RRF 融合后过滤）；None/空 = 全库检索。
    skill：App 校验后的技能 instructions；None = 不注入。
    """
    _any = collect_timing or collect_trace or collect_refs
    trace_id = start_trace()
    timing: dict = {}
    trace: dict = {}
    refs: list = []
    t0 = time.perf_counter()

    def _stage(name: str, counts: dict | None = None, detail: dict | None = None):
        if on_stage is not None:
            try:
                on_stage(name, counts or {}, int((time.perf_counter() - t0) * 1000), detail or {})
            except Exception:
                logger.warning(f"[stage] 回调失败: {name}", exc_info=True)

    def _ms(a, b):
        return f"{(b - a) * 1000:.1f}ms"

    def _rec(key, a, b):
        if collect_timing:
            timing[key] = round((b - a) * 1000, 1)

    def _tr_cand(c):
        if not isinstance(c, dict):
            return None
        rs = c.get("rerank_score")
        return {
            "chunk_id": c.get("chunk_id"),
            "document_id": c.get("document_id"),
            "file_name": c.get("file_name", ""),
            "score": round(float(c.get("score") or 0), 4),
            "rerank_score": round(float(rs), 4) if rs is not None else None,
            "sources": c.get("sources") or ([c.get("source")] if c.get("source") else []),
        }

    def _tr_list(key, items):
        if collect_trace:
            trace[key] = [x for x in (_tr_cand(c) for c in items) if x]

    try:
        logger.info(f">>> 查询开始: {question[:80]}")

        mode = answer_mode if answer_mode in ANSWER_MODE_BUDGETS else DEFAULT_ANSWER_MODE
        budget = ANSWER_MODE_BUDGETS[mode]
        expanded_query = question
        query_type = _detect_query_type(question)
        logger.info(f"  answer_mode: {mode} query_type: {query_type}")

        # ── 0.6 问题规划器 + 变体（并行；变体只基于原问题，不依赖改写）──
        with ThreadPoolExecutor(max_workers=2) as _pex:
            fut_plan = submit_with_trace(_pex, plan_query, question)
            fut_variants = submit_with_trace(
                _pex, generate_query_variants, question, question
            )
            plan, reject, reject_reason = fut_plan.result()
            _rec("规划改写", t0, time.perf_counter())
            pre_variants = fut_variants.result() or []
            _rec("变体生成", t0, time.perf_counter())

        if reject and reject_reason == "chitchat":
            logger.info(f"[PLANNER] chitchat 简短回复: {question[:60]}")
            _rec("规划改写", t0, time.perf_counter())
            if on_refs is not None:
                on_refs([])
            answer = generate_chitchat_answer(
                question, timing=timing if collect_timing else None, on_delta=on_delta,
            )
            _rec("总计", t0, time.perf_counter())
            _stage("generate_done", {"chars": len(answer)},
                   {"model": DEEPSEEK_MODEL, "ttft_ms": 0})
            if collect_trace:
                trace["query_type"] = "chitchat"
                trace["planner_multi"] = False
            return (answer, timing, trace, refs) if _any else answer

        if reject:
            logger.info(
                f"[PLANNER] 异常问题拒答 (reason={reject_reason}) | "
                f"规划耗时 {_ms(t0, time.perf_counter())}"
            )
            logger.info(">>> 查询完成（拒答）")
            _rec("规划改写", t0, time.perf_counter())
            _rec("总计", t0, time.perf_counter())
            if on_refs is not None:
                on_refs([])
            if collect_trace:
                trace["query_type"] = query_type
                trace["planner_multi"] = False
                trace["n_context"] = 0
                trace["context_chars"] = 0
            return (REFUSAL_ANSWER, timing, trace, refs) if _any else REFUSAL_ANSWER

        skill_fragment = _skill_prompt_fragment(skill)
        if skill_fragment:
            logger.info(f"[skill] 注入 {skill.get('name') or skill.get('id')} version={skill.get('version')}")
            _stage("skill_injected", {"skill": 1}, {"status": "applied"})
        elif skill_skipped:
            logger.warning("[skill] skill payload was present but skipped")
            _stage("skill_injected", {"skill": 0}, {"status": "skipped"})
        t_rewrite = time.perf_counter()
        _rec("规划改写", t0, t_rewrite)
        planner_multi = bool(plan and plan["type"] in ("parallel_subq", "multi_hop"))
        if planner_multi:
            query_type = "cross_doc"
        elif plan and plan["type"] == "single":
            planner_q = plan["steps"][0]["query"] or ""
            if planner_q and planner_q != question:
                logger.info(f"[PLANNER] 单步改写: {planner_q[:80]}")
                expanded_query = planner_q
        if planner_multi:
            t_faiss = t_bm25 = t_meta = t_sql = time.perf_counter()
            planner_exec = execute_plan(plan, {
                "faiss": faiss_searcher,
                "bm25": bm25_searcher,
                "resolve": _resolve_faiss_results,
                "fusion": fusion,
                "kb_ids": kb_ids,
            })
            merged = planner_exec.candidates
            merged = _filter_by_kb(merged, kb_ids)
            merged = _expand_siblings(merged)
            merged = _attach_doc_meta(merged)
            _stage("retrieval_done", {})
            t_rrf = time.perf_counter()
            _rec("RRF融合", t_rewrite, t_rrf)
            _stage("rrf_done", {"merged": len(merged)}, {"strategy": f"RRF (k={RRF_K})"})
            if collect_trace:
                trace["query_type"] = query_type
                trace["planner_multi"] = True
                _tr_list("faiss_hits", [])
                _tr_list("bm25_hits", [])
        else:
            variants = pre_variants
            if variants:
                logger.info(f"[MULTI-QUERY] 变体 {len(variants)} 条: {variants}")
            queries = [expanded_query] + variants
            embs = get_bge_embeddings(queries) if queries else None
            with ThreadPoolExecutor(max_workers=2 + 2 * len(queries)) as executor:
                if embs:
                    faiss_futures = [
                        submit_with_trace(executor, faiss_searcher.search_vec, emb, FAISS_TOP_K)
                        for emb in embs
                    ]
                else:
                    faiss_futures = [
                        submit_with_trace(executor, faiss_searcher.search, q, FAISS_TOP_K)
                        for q in queries
                    ]
                bm25_futures = [
                    submit_with_trace(executor, bm25_searcher.search, q, BM25_TOP_K)
                    for q in queries
                ] if (bm25_searcher and bm25_searcher.ready) else []

                faiss_hits = _merge_faiss_multi([f.result() for f in faiss_futures])
                t_faiss = time.perf_counter()
                bm25_results = (
                    _merge_bm25_multi([f.result() for f in bm25_futures])
                    if bm25_futures else []
                )
                t_bm25 = time.perf_counter()
                t_meta = t_sql = t_bm25
            _rec("检索_faiss", t_rewrite, t_faiss)
            _rec("检索_bm25", t_rewrite, t_bm25)

            logger.info(
                f"[并行检索] faiss={len(faiss_hits)}条 bm25={len(bm25_results)}条 "
                f"| 并行总耗时 {_ms(t_rewrite, t_bm25)}"
            )

            faiss_results = _resolve_faiss_results(faiss_hits)
            _stage("retrieval_done", {"faiss": len(faiss_results), "bm25": len(bm25_results)})
            if collect_trace:
                trace["query_type"] = query_type
                trace["planner_multi"] = False
                _tr_list("faiss_hits", faiss_results)
                _tr_list("bm25_hits", bm25_results)
            merged = fusion.rrf_fusion(faiss_results, None, [], bm25_results)
            merged = _filter_by_kb(merged, kb_ids)
            merged = _expand_siblings(merged)
            merged = _attach_doc_meta(merged)
            t_rrf = time.perf_counter()
            _rec("RRF融合", t_bm25, t_rrf)
            _stage("rrf_done", {"merged": len(merged)}, {"strategy": f"RRF (k={RRF_K})"})

        # ── 5. Reranker 精排 + 源配额 ──
        _stage("rerank_start")
        # top_k 留补位余量：上下文组装会跳过与已选 parent 重叠的 child，需要候选补位
        # 规划路径：对原问题 + 各 step 检索文本多路打分取最大值（防第二问语义稀释）
        # A/B 开关：RERANK_SKIP_SINGLE=1 时单步问题跳过精排 API（RRF 排序直出）
        reranked = fusion.rerank(
            question, merged, top_k=FINAL_TOP_K + 4, query_type=query_type,
            sub_queries=planner_exec.queries if planner_multi else None,
            skip_api=(
                RERANK_SKIP_SINGLE and not planner_multi
                and bool(plan) and plan["type"] == "single"
                and query_type == "single_doc"
            ),
        )
        # 生命周期检索语义(§2.14 A)：废止文档降权 + 时间感知，重排后调整
        reranked = _apply_lifecycle(question, reranked, include_expired)
        # 邻域补全：入选 chunk 的同文档相邻块（±N）若被精排裁剪，按融合序补入尾部。
        # 法规条款/补贴标准类细节题的正确 chunk 常与已选 chunk 相邻但 rerank 分数低。
        reranked = _append_neighbors(reranked, merged)
        if not include_expired:
            reranked = _drop_expired(reranked)
        scored = [c.get("rerank_score") for c in reranked if c.get("rerank_score") is not None]
        if scored and max(scored) < ANSWER_MIN_RERANK_SCORE:
            logger.info(
                f"[证据门槛] 最高精排分 {max(scored):.4f} < {ANSWER_MIN_RERANK_SCORE}，按零命中处理"
            )
            reranked = []
        # Final pre-context / pre-reference recheck: neighbor expansion must not
        # reintroduce a document the caller cannot read.
        t_rerank = time.perf_counter()
        _rec("精排rerank", t_rrf, t_rerank)
        image_candidates = _collect_image_candidates(reranked)
        text_reranked = [
            c for c in reranked if (c.get("chunk_type") or "text") != "image"
        ]
        _stage("rerank_done", {
            "reranked": len(text_reranked), "images": len(image_candidates),
        }, _rerank_detail(text_reranked))
        if on_images is not None and image_candidates:
            on_images(image_candidates)
        if collect_refs or on_refs is not None:
            refs = _build_refs(text_reranked)
            if on_refs is not None:
                on_refs(refs)
        _tr_list("rrf_merged", merged)
        _tr_list("reranked", text_reranked)
        logger.info(
            f"  精排: {len(text_reranked)} 条文本 / {len(image_candidates)} 张图片"
        )
        for i, c in enumerate(text_reranked, 1):
            logger.info(
                f"    [{i}] source={c.get('source')} chunk_id={c.get('chunk_id')} "
                f"doc_id={c.get('document_id')} 文号={c.get('doc_number', '')} "
                f"rerank={c.get('rerank_score')} srcs={','.join(c.get('sources') or [])} "
                f"文件={str(c.get('file_name', ''))[:60]}"
            )

        # ── 6. 取 Parent Chunk 上下文 ──
        contexts = _expand_to_parent_contexts(
            text_reranked, planner_multi=planner_multi,
        )
        if not contexts and image_candidates:
            contexts = [
                f"[图片说明] {img.get('caption') or ''}\n{img.get('ocr') or ''}".strip()
                for img in image_candidates
            ]
        if collect_trace:
            trace["n_context"] = len(contexts)
            trace["context_chars"] = sum(len(c) for c in contexts)
        t_ctx = time.perf_counter()
        _rec("parent上下文", t_rerank, t_ctx)
        _stage("context_done", {"contexts": len(contexts), "chars": sum(len(c) for c in contexts)},
               {"max_chars": budget["context_chars"]})
        for i, ctx in enumerate(contexts, 1):
            preview = ctx[:200].replace("\n", " / ")
            logger.info(f"  召回上下文[{i}] ({len(ctx)}字符): {preview}...")

        # ── 7. LLM 生成（基础提示词 + 学习/复习模板 + 技能 + 语言规则）──
        _stage("generate_start", None, {"model": DEEPSEEK_MODEL, "answer_mode": mode})
        style = compute_answer_style(text_reranked or reranked, question)
        logger.info(
            f"[PROMPT] 模式={mode} 文档语言={style['doc_lang']} "
            f"提问语言={style['answer_lang']} 跨语言={'是' if style['cross_lang'] else '否'}"
        )
        t_gen = time.perf_counter()
        ttft_ms = {"v": 0}

        def _delta_ttft(txt):
            # 首 token 延迟（TTFT）：从 generate_start 到第一个增量到达
            if not ttft_ms["v"]:
                ttft_ms["v"] = int((time.perf_counter() - t_gen) * 1000)
            if on_delta is not None:
                on_delta(txt)

        if planner_multi:
            answer = generate_synthesis_answer(
                question, planner_exec.intermediates, contexts, style,
                answer_mode=mode, timing=timing if collect_timing else None,
                on_delta=_delta_ttft, extra_rules=skill_fragment,
                images=image_candidates,
            )
        else:
            answer = generate_answer(
                question, contexts, style, answer_mode=mode,
                timing=timing if collect_timing else None,
                on_delta=_delta_ttft, extra_rules=skill_fragment,
                images=image_candidates,
            )
        t_llm = time.perf_counter()
        _rec("LLM生成", t_ctx, t_llm)
        _rec("总计", t0, t_llm)

        # ── 各阶段耗时汇总 ──
        logger.info(
            f"  耗时: 重写/规划 {_ms(t0, t_rewrite)} | "
            f"并行检索 faiss {_ms(t0, t_faiss)} / bm25 {_ms(t0, t_bm25)} | "
            f"RRF {_ms(t_bm25, t_rrf)} | 精排 {_ms(t_rrf, t_rerank)} | "
            f"Parent {_ms(t_rerank, t_ctx)} | LLM {_ms(t_ctx, t_llm)} | "
            f"总计 {_ms(t0, t_llm)}"
        )

        answer = _replace_image_tokens(answer, image_candidates)
        logger.info(
            f">>> 查询完成: 答案 {len(answer)} 字 / 图片 {len(image_candidates)} 张"
        )

        # 答案生成后对齐引用位置（内联引用芯片用），done 事件携带
        if _any and refs and answer:
            _align_ref_positions(answer, refs, text_reranked)
            positioned = sum(1 for r in refs if r.get("start_at") is not None)
            if positioned:
                logger.info(f"  引用对齐: {positioned}/{len(refs)} 条命中位置")

        _stage("generate_done", {
            "chars": len(answer) if answer else 0, "images": len(image_candidates),
        }, {"model": DEEPSEEK_MODEL, "ttft_ms": ttft_ms["v"]})

        return (answer, timing, trace, refs) if _any else answer

    except Exception as e:
        logger.error(f"Query 处理异常: {e}", exc_info=True)
        msg = "系统处理出错，请重试"
        if on_refs is not None:
            on_refs([])
        return (msg, timing, trace, refs) if _any else msg
    finally:
        clear_trace()


@app.route("/chat", methods=["POST"])
def handle_chat():
    """竞赛标准接口：POST /chat，参数 question（JSON 或表单均可），返回 {"answer": ...}。

    JSON 请求带 "timing": true 时响应附 "timings"（各阶段耗时，毫秒）；
    带 "trace": true 时响应附 "trace"（检索链中间数据，评测用）。
    """
    timing_requested = False
    trace_requested = False
    kb_ids = None
    skill = None
    skill_skipped = False
    include_expired = False
    answer_mode = DEFAULT_ANSWER_MODE
    if request.is_json:
        body = request.get_json(silent=True) or {}
        question = body.get("question")
        timing_requested = bool(body.get("timing"))
        trace_requested = bool(body.get("trace"))
        if body.get("knowledge_base_id"):
            kb_ids = [body["knowledge_base_id"]]
        elif body.get("knowledge_base_ids"):
            kb_ids = list(body["knowledge_base_ids"])
        skill = _parse_skill(body)
        skill_skipped = bool(body.get("skill")) and skill is None
        include_expired = bool(body.get("include_expired"))
        answer_mode = (body.get("answer_mode") or DEFAULT_ANSWER_MODE).strip().lower()
    else:
        question = request.form.get("question") or request.args.get("question")
    question = (question or "").strip()
    if not question:
        return jsonify({"answer": "请输入您的问题。"})
    if timing_requested or trace_requested:
        answer, timings, trace, _refs = _run_query(
            question, collect_timing=timing_requested, collect_trace=trace_requested,
            kb_ids=kb_ids, skill=skill,
            skill_skipped=skill_skipped, include_expired=include_expired,
            answer_mode=answer_mode,
        )
        resp = {"answer": answer}
        if timing_requested:
            resp["timings"] = timings
        if trace_requested:
            resp["trace"] = trace
        return jsonify(resp)
    return jsonify({"answer": _run_query(
        question, kb_ids=kb_ids, skill=skill,
        skill_skipped=skill_skipped, include_expired=include_expired,
        answer_mode=answer_mode,
    )})


def _collect_image_candidates(reranked: list[dict]) -> list[dict]:
    """收集可直接展示的图片：图片块命中 + 文本块关联图片，按 URL 去重。"""
    out: list[dict] = []
    seen: set[str] = set()

    def add(row: dict, fallback_source: str = "") -> None:
        url = str(row.get("image_url") or row.get("url") or "").strip()
        if not url or url in seen:
            return
        seen.add(url)
        out.append({
            "url": url,
            "caption": str(row.get("image_caption") or row.get("caption") or "").strip(),
            "ocr": str(row.get("image_ocr") or row.get("ocr_text") or "").strip(),
            "file_name": str(row.get("file_name") or "").strip(),
            "doc_title": str(row.get("doc_title") or "").strip(),
            "source_chunk_id": str(
                row.get("parent_source_chunk_id") or fallback_source or ""
            ),
        })

    for c in reranked:
        if (c.get("chunk_type") or "text") == "image":
            add(c)

    if chunk_store:
        for c in reranked:
            if (c.get("chunk_type") or "text") == "image":
                continue
            source_id = (
                c.get("source_chunk_id")
                or chunk_store.source_chunk_id(c.get("chunk_id"))
            )
            for row in chunk_store.images_for_source_chunk(source_id):
                add(row, str(source_id or ""))

    return out


_IMAGE_TOKEN_RE = re.compile(r"\[\[IMG:(\d+)\]\]")


def _image_markdown(img: dict) -> str:
    url = str(img.get("url") or "").strip()
    if not url:
        return ""
    caption = str(img.get("caption") or "").strip()
    alt = caption.replace("[", "\\[").replace("]", "\\]") or "图片"
    lines = [f"![{alt}]({url})"]
    meta = []
    if caption:
        meta.append(f"图注：{caption}")
    if img.get("file_name"):
        meta.append(f"来源：{img['file_name']}")
    if meta:
        lines.append("> " + " · ".join(meta))
    return "\n".join(lines)


def _replace_image_tokens(text: str, images: list[dict]) -> str:
    if not text or not images:
        return text

    def _sub(match):
        try:
            idx = int(match.group(1))
        except (TypeError, ValueError):
            return ""
        if idx < 1 or idx > len(images):
            return ""
        return _image_markdown(images[idx - 1])

    return _IMAGE_TOKEN_RE.sub(_sub, text)


class StreamingImageReplacer:
    """SSE 增量替换 [[IMG:n]]；跨 delta 的半截 token 先暂存。"""

    _HOLD_PREFIX = "[[IMG:"

    def __init__(self, images: list[dict]):
        self.images = images
        self.buf = ""

    def feed(self, text: str) -> str:
        self.buf += text or ""
        hold = 0
        max_prefix = len(self._HOLD_PREFIX) + 4
        for i in range(len(self.buf) - 1, max(-1, len(self.buf) - max_prefix - 1), -1):
            suffix = self.buf[i:]
            if suffix.startswith("[") and (
                self._HOLD_PREFIX.startswith(suffix)
                or re.fullmatch(r"\[\[IMG:\d*", suffix)
            ):
                hold = len(suffix)
                break
        if hold:
            out, self.buf = self.buf[:-hold], self.buf[-hold:]
        else:
            out, self.buf = self.buf, ""
        return _replace_image_tokens(out, self.images)

    def flush(self) -> str:
        out = _replace_image_tokens(self.buf, self.images)
        self.buf = ""
        return out


def _build_refs(reranked: list[dict]) -> list[dict]:
    """精排结果 → app fusion 客户端期望的引用结构（YLSWReference）。

    id 一律转字符串（Go 端结构体字段为 string，数字会反序列化失败）。
    lifecycle_status 透出（§2.14 portal 引用「已废止」标签）。
    content 为 chunk 原文摘要（前端悬浮引用卡直接展示，不回查 chunk 接口）。
    """
    refs = []
    for c in reranked:
        cid = c.get("chunk_id")
        if cid in (None, -1, "", "-1"):
            continue
        if (c.get("chunk_type") or "text") == "image":
            continue
        text = c.get("text") or ""
        # 去掉 chunk 开头的 "# 文档标题 > ## 章节" 标题行，悬浮卡直接展示正文
        lines = text.splitlines()
        while lines and (lines[0].startswith("#") or lines[0].startswith(">")):
            lines.pop(0)
        clean = "\n".join(lines).strip()
        excerpt = clean[:400] + ("…" if len(clean) > 400 else "")
        doc_meta = chunk_store.doc_meta(c.get("document_id")) if chunk_store else {}
        refs.append({
            "chunk_id": str(cid),
            "app_chunk_id": chunk_store.source_chunk_id(cid) if chunk_store else "",
            "document_id": str(c.get("document_id", "")),
            "source_doc_id": c.get("source_doc_id", "") or doc_meta.get("source_doc_id", ""),
            "knowledge_base_id": c.get("knowledge_base_id", "") or "",
            "file_name": c.get("file_name", ""),
            "doc_number": c.get("doc_number", ""),
            "org_name": c.get("org_name", ""),
            "doc_type": c.get("doc_type", ""),
            "product_name": c.get("product_name", ""),
            "doc_title": c.get("doc_title", ""),
            "content": excerpt,
            "doc_date": str(c.get("doc_date", "") or ""),
            "source": c.get("source", ""),
            "lifecycle_status": c.get("lifecycle_status", ""),
            "rerank_score": c.get("rerank_score"),
        })
    return refs


_SENT_SPLIT_RE = re.compile(r"[。；;!?！？\n]+")
_MIN_ALIGN_CHARS = 10


def _find_align_span(answer: str, text: str):
    """在 answer 中定位 chunk 原文的最长可命中片段，返回 (start, end) 字符偏移。

    按句切分 chunk 文本，逐句先精确查找，未中再用最长公共子串（容忍 LLM 改写）。
    取最长命中（引用越具体越可信）；无命中返回 None（该条不产生内联位置）。
    """
    if not answer or not text:
        return None
    best = None
    for sent in _SENT_SPLIT_RE.split(text):
        frag = re.sub(r"\s+", "", sent)
        if len(frag) < _MIN_ALIGN_CHARS:
            continue
        pos = answer.find(frag)
        if pos >= 0:
            size = len(frag)
        else:
            m = SequenceMatcher(None, answer, frag, autojunk=False)
            block = max(m.get_matching_blocks(), key=lambda b: b.size)
            if block.size < _MIN_ALIGN_CHARS:
                continue
            size, pos = block.size, block.a
        if best is None or size > best[0]:
            best = (size, pos)
    if best is None:
        return None
    size, pos = best
    return (pos, pos + size)


def _align_ref_positions(answer: str, refs: list[dict], reranked: list[dict]) -> None:
    """答案文本与引用 chunk 原文对齐，回填 start_at/end_at（字符偏移）。

    位置供 app 前端插入内联 [n] 上标；无命中则不留位置，
    引用面板/底部列表只展示有位置的引用。命中后把 content 摘要窗口
    对准命中句（前后各留余量），保证悬浮卡里能高亮对应原文。
    同位置冲突时先到先得，避免同一处叠多个标记。
    """
    text_map = {}
    for c in reranked:
        cid = c.get("chunk_id")
        txt = c.get("text") or ""
        if cid is not None and txt:
            text_map[str(cid)] = txt
    used: list[tuple[int, int]] = []
    for r in refs:
        txt = text_map.get(r.get("chunk_id", ""))
        span = _find_align_span(answer, txt) if txt else None
        if span is None:
            continue
        s, e = span
        if any(not (e <= us or s >= ue) for us, ue in used):
            continue
        used.append((s, e))
        r["start_at"] = s
        r["end_at"] = e
        # 摘要窗口对准命中句（命中句可能因空白归一化与原文不完全一致，取不到就保留原窗口）
        frag = re.sub(r"\s+", "", answer[s:e])
        idx = txt.find(frag) if frag else -1
        if idx >= 0:
            window = txt[max(0, idx - 120): idx + len(frag) + 280]
            lines = window.splitlines()
            while lines and (lines[0].startswith("#") or lines[0].startswith(">")):
                lines.pop(0)
            clean = "\n".join(lines).strip()
            if clean:
                r["content"] = clean + ("…" if idx + len(frag) + 280 < len(txt) else "")


def _rerank_detail(reranked: list[dict]) -> dict:
    """重排阶段排障明细：后端/阈值/Top 候选（前端 trace 行展开）。"""
    tops = []
    for c in reranked[:3]:
        lines = (c.get("text") or "").splitlines()
        while lines and (lines[0].startswith("#") or lines[0].startswith(">")):
            lines.pop(0)
        snippet = re.sub(r"\s+", " ", "\n".join(lines)).strip()[:60]
        tops.append({
            "score": round(float(c.get("rerank_score") or 0), 3),
            "doc": (c.get("file_name") or "")[:40],
            "text": snippet,
        })
    return {
        "backend": BGE_RERANK_BACKEND,
        "threshold": RERANK_META_MIN_SCORE,
        "top": tops,
    }


@app.route("/chat/stream", methods=["POST"])
def handle_chat_stream():
    """app(fusion) 流式接口：SSE 事件流，协议对齐 app/internal/fusion/ylsw_client.go。

    事件类型（data: 行内 JSON）：
      references: {"type":"references","references":[YLSWReference...]} —— 检索完成先发
      delta:      {"type":"delta","content":"..."} —— 答案增量
      done:       {"type":"done","references":[...]} —— 结束（携带最终引用）
      error:      {"type":"error","message":"..."} —— 出错

    knowledge_base_id / knowledge_base_ids：KB 白名单，按 chunks.knowledge_base_id
    过滤（单值兼容内部问答，数组供门户多 KB 检索；空 = 全库检索）。
    """
    body = request.get_json(silent=True) or {}
    question = (body.get("question") or "").strip()
    if not question:
        question = (request.form.get("question") or request.args.get("question") or "").strip()
    knowledge_base_id = (body.get("knowledge_base_id") or "").strip()
    knowledge_base_ids = body.get("knowledge_base_ids") or []
    skill = _parse_skill(body)
    skill_skipped = bool(body.get("skill")) and skill is None
    include_expired = bool(body.get("include_expired"))
    answer_mode = (body.get("answer_mode") or DEFAULT_ANSWER_MODE).strip().lower()
    if not question:
        return jsonify({"answer": "请输入您的问题。"})

    kb_ids = None
    if knowledge_base_ids:
        kb_ids = [str(k).strip() for k in knowledge_base_ids if str(k).strip()]
    elif knowledge_base_id:
        kb_ids = [knowledge_base_id]
    if kb_ids:
        logger.info(f"[fusion] kb_ids={kb_ids}（KB 过滤已启用）")

    def generate():
        q: queue.Queue = queue.Queue()

        def _refs(r):
            q.put(("references", r))

        def _delta(t):
            q.put(("delta", t))

        def _stage(name, counts, t_ms, detail):
            q.put(("stage", {"stage": name, "t_ms": t_ms, "counts": counts or {}, "detail": detail or {}}))

        def _images(images):
            q.put(("images", images))

        def _worker():
            try:
                q.put(("result", _run_query(
                    question, collect_refs=True, on_refs=_refs, on_delta=_delta,
                    on_images=_images, on_stage=_stage,
                    kb_ids=kb_ids, skill=skill,
                    skill_skipped=skill_skipped, include_expired=include_expired,
                    answer_mode=answer_mode,
                )))
            except Exception as e:
                logger.error(f"[fusion] 流式查询异常: {e}", exc_info=True)
                q.put(("error", f"系统处理出错: {e}"))

        threading.Thread(target=_worker, daemon=True, name="fusion-query").start()

        refs_sent = False
        delta_sent = False
        final_refs = []
        result = None
        image_replacer = None
        while True:
            kind, payload = q.get()
            if kind == "error":
                yield "data: " + json.dumps(
                    {"type": "error", "message": payload}, ensure_ascii=False,
                ) + "\n\n"
                return
            if kind == "references":
                refs_sent = True
                final_refs = payload
                yield "data: " + json.dumps(
                    {"type": "references", "references": payload},
                    ensure_ascii=False,
                ) + "\n\n"
            elif kind == "images":
                image_replacer = StreamingImageReplacer(payload)
            elif kind == "delta":
                content = image_replacer.feed(payload) if image_replacer else payload
                if not content:
                    continue
                delta_sent = True
                yield "data: " + json.dumps(
                    {"type": "delta", "content": content}, ensure_ascii=False,
                ) + "\n\n"
            elif kind == "stage":
                yield "data: " + json.dumps(
                    {"type": "stage", **payload}, ensure_ascii=False,
                ) + "\n\n"
            elif kind == "result":
                result = payload
                break

        answer = result[0] if isinstance(result, tuple) else result
        if image_replacer:
            tail = image_replacer.flush()
            if tail:
                delta_sent = True
                yield "data: " + json.dumps(
                    {"type": "delta", "content": tail}, ensure_ascii=False,
                ) + "\n\n"

        # 兜底：拒答/LLM 失败等路径没有回调，补发整段答案，保证前端有内容
        if not refs_sent:
            yield "data: " + json.dumps(
                {"type": "references", "references": final_refs},
                ensure_ascii=False,
            ) + "\n\n"
        if not delta_sent and answer:
            yield "data: " + json.dumps(
                {"type": "delta", "content": answer}, ensure_ascii=False,
            ) + "\n\n"
        yield "data: " + json.dumps(
            {"type": "done", "references": final_refs}, ensure_ascii=False,
        ) + "\n\n"

    return Response(
        stream_with_context(generate()),
        mimetype="text/event-stream",
        headers={
            "Cache-Control": "no-cache",
            "X-Accel-Buffering": "no",
        },
    )


_CROSS_DOC_SIGNALS = (
    "对比", "比较", "区别", "差异", "哪些", "所有", "全部",
    "汇总", "全景", "分别", "各自", "综述", "梳理",
    "关系", "联系", "关联", "异同", "相同", "不同",
)


def _detect_query_type(question: str) -> str:
    """根据问法特征判断单文档还是跨文档检索。"""
    if any(signal in question for signal in _CROSS_DOC_SIGNALS):
        return "cross_doc"
    if "2023" in question and "2024" in question:
        return "cross_doc"
    return "single_doc"


def _resolve_faiss_results(faiss_hits: list) -> list[dict]:
    """将 FAISS 返回的 (vector_id, score) 转为含文本+元数据的结果（内存直查）。"""
    if not faiss_hits or not chunk_store:
        return []
    return chunk_store.resolve(faiss_hits)


def _merge_faiss_multi(hits_lists: list[list[tuple[int, float]]]) -> list[tuple[int, float]]:
    """多查询 FAISS 结果合并：按 vector_id 去重取最高 score，降序重排。"""
    best: dict[int, tuple[int, float]] = {}
    for hits in hits_lists:
        for vid, score in hits:
            if vid not in best or score > best[vid][1]:
                best[vid] = (vid, score)
    return sorted(best.values(), key=lambda x: x[1], reverse=True)


def _merge_bm25_multi(hits_lists: list[list[dict]]) -> list[dict]:
    """多查询 BM25 结果合并：按 chunk id 去重取最高 score，降序重排。"""
    best: dict = {}
    for hits in hits_lists:
        for item in hits:
            key = item.get("id") or item.get("chunk_id")
            if key is None:
                continue
            if key not in best or item.get("score", 0) > best[key].get("score", 0):
                best[key] = item
    return sorted(best.values(), key=lambda x: x.get("score", 0), reverse=True)


_VERSION_RE = re.compile(r"V(\d+)[._](\d+)(?:[._]\d+)*")  # 剥整段版本号 V4.2.7/V1_0
_VERSION_PARSE_RE = re.compile(r"V(\d+)[._](\d+)")        # 取前两位比较


def _parse_version(file_name: str) -> tuple[int, int]:
    """文件名版本号 V数字_数字 → (大版本, 小版本)；无版本号 (0, 0)。"""
    m = _VERSION_PARSE_RE.search(file_name or "")
    return (int(m.group(1)), int(m.group(2))) if m else (0, 0)


def _doc_group_key(file_name: str) -> str:
    """同文档不同版本归组键：去版本号、语言后缀、类型日期前缀。"""
    name = _VERSION_RE.sub("", file_name or "")
    name = re.sub(r"_(ZH|EN)\.md$", "", name)
    name = re.sub(r"^(REG|MAN|NWS|PDT|RPT|TBL|GEN)_\d{8}_", "", name)
    return name


def _dedup_by_latest_version(candidates: list[dict]) -> list[dict]:
    """元数据去重：同文档多版本同时命中召回时，仅保留版本最新文档的候选。

    版本比较：先比第一个数字，相同再比第二个数字（V4.2.7 → (4,2)）。
    输出保持输入原始顺序（多文档题各组交错，聚合重排会打乱重排序位）。
    """
    groups: dict[str, list[tuple]] = {}
    for c in candidates:
        fn = c.get("file_name") or ""
        groups.setdefault(_doc_group_key(fn), []).append((_parse_version(fn), c))

    kept_ids: set[int] = set()
    for key, items in groups.items():
        if len(items) == 1:
            kept_ids.add(id(items[0][1]))
            continue
        versions = {v for v, _ in items}
        if len(versions) == 1:
            kept_ids.update(id(c) for _, c in items)
            continue
        max_ver = max(versions)
        kept = [c for v, c in items if v == max_ver]
        kept_ids.update(id(c) for c in kept)
        logger.info(
            f"[去重] 文档 {key}: {len(items)} 条候选跨 {len(versions)} 个版本，"
            f"仅保留 V{max_ver[0]}_{max_ver[1]}（剔除 {len(items) - len(kept)} 条旧版）"
        )
    return [c for c in candidates if id(c) in kept_ids]


_CJK_RE = re.compile(r"[一-鿿]")


def detect_language(text: str) -> str:
    """CJK 字符占比 >0.5 判中文，否则英文（混合中英问题时优先英文）。"""
    if not text:
        return "zh"
    letters = sum(1 for ch in text if ch.isalnum())
    cjk = len(_CJK_RE.findall(text))
    return "zh" if letters == 0 or cjk / letters > 0.5 else "en"


def compute_answer_style(reranked: list[dict], question: str) -> dict:
    """统计召回候选元数据 → 语言适配参数。

    - 文档语言按 file_name _ZH/_EN 取多数（并列取先出现者）
    - answer_lang = 提问语言；cross_lang = 文档语言 ≠ 提问语言
    """
    style = {
        "answer_lang": detect_language(question),
        "doc_lang": None,
        "cross_lang": False,
    }

    doc_cands = [c for c in reranked if c.get("file_name")]
    if not doc_cands:
        return style

    # 文档语言：_ZH/_EN 多数
    lang_votes = {"zh": 0, "en": 0}
    first_lang = None
    for c in doc_cands:
        fn = c.get("file_name") or ""
        lang = "en" if fn.endswith("_EN.md") else "zh"
        lang_votes[lang] += 1
        if first_lang is None:
            first_lang = lang
    style["doc_lang"] = (
        "zh" if lang_votes["zh"] >= lang_votes["en"] else "en"
    ) if lang_votes["zh"] != lang_votes["en"] else first_lang
    style["cross_lang"] = style["doc_lang"] != style["answer_lang"]
    return style


def _expand_siblings(merged: list[dict], radius: int = 3,
                     max_add: int = 8) -> list[dict]:
    """检索邻域扩展：融合池中每个语料候选的同文档相邻 child 补入池尾部，
    补偿语义检索对相邻条款块（如补贴标准的分段条款）的漏召回。"""
    if not chunk_store:
        return merged
    out = list(merged)
    seen = {str(c.get("chunk_id")) for c in merged}
    added = 0
    for c in merged:
        if added >= max_add:
            break
        doc = c.get("document_id")
        ps = c.get("position_start")
        if not (isinstance(doc, int) and isinstance(ps, int)):
            continue
        for r in chunk_store.neighbors(doc, ps, radius):
            if str(r["id"]) in seen:
                continue
            seen.add(str(r["id"]))
            out.append({
                "id": r["id"],
                "chunk_id": r["id"],
                "text": r["chunk_text"],
                "source": "faiss",
                "document_id": r["document_id"],
                "file_name": r.get("file_name", ""),
                "org_name": r.get("org_name", ""),
                "doc_type": r.get("doc_type", ""),
                "product_name": r.get("product_name", ""),
                "knowledge_base_id": r.get("knowledge_base_id", ""),
                "doc_number": r.get("doc_number", ""),
                "doc_date": str(r.get("doc_date", "") or ""),
                "doc_title": r.get("doc_title", ""),
                "position_start": r.get("position_start"),
                "position_end": r.get("position_end"),
            })
            added += 1
            if added >= max_add:
                break
    if added:
        logger.info(f"[邻域扩展] 融合池补入 {added} 条相邻块")
    return out


def _append_neighbors(reranked: list[dict], merged: list[dict],
                      radius: int = 8, max_extra: int = 8) -> list[dict]:
    """邻域补全：把融合链中与已入选 chunk 同文档相邻（chunk 序号差 ≤ radius）
    但被精排裁剪的候选补入重排列表尾部，防条款类细节丢失。"""
    selected = []
    for c in reranked:
        doc = c.get("document_id")
        ps = c.get("position_start")
        if not (isinstance(doc, int) and isinstance(ps, int)):
            continue
        selected.append((doc, ps))
    if not selected:
        return reranked

    out = list(reranked)
    seen_ids = {str(c.get("chunk_id")) for c in reranked}
    extra = 0
    for c in merged:
        if extra >= max_extra:
            break
        cid = str(c.get("chunk_id"))
        if cid in seen_ids:
            continue
        doc = c.get("document_id")
        ps = c.get("position_start")
        if not (isinstance(doc, int) and isinstance(ps, int)):
            continue
        if any(doc == d and abs(ps - p) <= radius for d, p in selected):
            c = dict(c)
            c["rerank_score"] = None  # 补位候选无精排分，置尾部
            out.append(c)
            seen_ids.add(cid)
            extra += 1
    if extra:
        logger.info(f"[邻域补全] 补入 {extra} 条相邻候选")
    return out


def _prepend_doc_summaries(contexts: list[str], doc_ids: list[int]) -> list[str]:
    """多步题涉及的文档要点摘要前置到上下文最前（仅 planner_multi）。"""
    summaries = []
    for d in doc_ids[:3]:
        s = chunk_store.summary_of(d) if chunk_store else None
        if s:
            summaries.append(f"【文档摘要】{s}")
    if summaries:
        logger.info(f"[摘要前置] {len(summaries)} 篇文档摘要加入上下文头部")
    return summaries + contexts


def _expand_to_parent_contexts(reranked: list[dict], max_contexts: int = FINAL_TOP_K,
                               planner_multi: bool = False) -> list[str]:
    """分层上下文组装：每源第 1 条取 parent chunk（完整上下文），其余取 child chunk（节省 token）。

    去重：child 完全落在已收录 span 内（同文档）时跳过——Top-1 的 parent 已覆盖其内容，
    跳过后由后续候选补位，最多组装 max_contexts 条不重复上下文。

    合并：同文档上下文 span 重叠或相邻（相接）时合并为一条——取并集区间的 child 单元
    文本重建（无重叠、无遗漏），沿用首条标签。跨章节（span 不相接）不合并，
    保证列表式问题各章节各自成条。

    """
    # 元数据去重：同文档多版本召回只留最新版
    reranked = _dedup_by_latest_version(reranked)

    if not chunk_store:
        return [c["text"] for c in reranked[:max_contexts]]

    TYPE_LABELS = {
        "regulation": "法规", "manual": "手册", "product": "产品", "report": "报告",
        "news": "新闻", "table": "表格", "general": "通用",
    }

    def _build_label(item: dict) -> str:
        meta_parts = []
        if item.get("doc_number"):
            meta_parts.append(f"文号:{item['doc_number']}")
        if item.get("org_name"):
            meta_parts.append(f"机构:{item['org_name']}")
        if item.get("doc_type"):
            label = TYPE_LABELS.get(item["doc_type"], item["doc_type"])
            meta_parts.append(f"类型:{label}")
        if item.get("product_name"):
            meta_parts.append(f"产品:{item['product_name']}")
        if item.get("lifecycle_status") == "expired":
            meta_parts.append("状态:已废止")
        if item.get("file_name"):
            meta_parts.append(f"来源:{item['file_name']}")
        return f"[{' | '.join(meta_parts)}]" if meta_parts else ""

    def _fetch_union_text(doc_id: int, start: int, end: int) -> str:
        """取 [start, end) 区间内全部 child 单元文本按序拼接（内存直查）。"""
        return chunk_store.union_text(doc_id, start, end)

    entries: list[dict] = []  # chunk 含 doc_id/start/end/label
    seen = set()
    source_count: dict[str, int] = {}

    for item in reranked:
        chunk_id = item.get("chunk_id")
        if not chunk_id or chunk_id == -1 or chunk_id in seen:
            continue

        # parent 反查走内存预计算映射（启动时已算好，无 DB 查询）
        row = chunk_store.parent_of(chunk_id)

        if not row:
            continue

        doc_id = row["document_id"]
        c_start, c_end = row["position_start"], row["position_end"]

        # 去重：child 完全落在已收录 span 内 → 与已选 parent/child 内容重复，跳过。
        # 仅当已收录条目是完整文本（top1 parent 或合并后 union 全文）时才可据此跳过；
        # 若已收录条目只是局部 child 文本，其 span 不构成覆盖证据，不得跳过。
        if any(
            e["kind"] == "chunk" and e["doc_id"] == doc_id
            and e["start"] <= c_start and c_end <= e["end"]
            and e.get("full")
            for e in entries
        ):
            continue

        seen.add(chunk_id)
        is_top1 = not any(
            e["kind"] == "chunk" and e["doc_id"] == doc_id for e in entries
        )
        label = _build_label(item)

        if is_top1 and row.get("parent_text") is not None:
            # parent 瘦身：截 1500 保留章节主体（尾部多为冗余操作细节）
            parent_text = row["parent_text"][:1500]
            text = f"{label}\n{parent_text}" if label else parent_text
            span = (row["p_start"], row["p_end"])
            is_full = True
        else:
            text = item.get("text", "")
            text = f"{label}\n{text}" if label else text
            span = (c_start, c_end)
            is_full = False

        # 合并：与同文档已有条目 span 重叠或相接 → 并集区间重建正文
        merged = False
        for e in entries:
            if e["kind"] != "chunk" or e["doc_id"] != doc_id:
                continue
            if span[0] <= e["end"] and e["start"] <= span[1]:
                us, ue = min(e["start"], span[0]), max(e["end"], span[1])
                body = _fetch_union_text(doc_id, us, ue)
                if body:
                    keep_label = e["label"] if e["start"] <= span[0] else label
                    e["start"], e["end"] = us, ue
                    e["text"] = f"{keep_label}\n{body}" if keep_label else body
                    e["full"] = True  # union 重建为区间完整文本
                    logger.info(
                        f"  合并上下文: doc_id={doc_id} 并集 [{us},{ue}) "
                        f"(吸收 span [{span[0]},{span[1]})，省 1 条)"
                    )
                merged = True
                break
        if not merged:
            entries.append({
                "kind": "chunk", "doc_id": doc_id,
                "start": span[0], "end": span[1],
                "text": text, "label": label, "full": is_full,
            })

        if len(entries) >= max_contexts:
            break

    total_chars = sum(len(e["text"]) for e in entries)
    logger.info(f"[上下文] 组装 {len(entries)} 条 共 {total_chars} 字符")
    texts = [e["text"] for e in entries]
    if planner_multi:
        doc_ids = []
        for e in entries:
            if e["kind"] == "chunk" and e["doc_id"] not in doc_ids:
                doc_ids.append(e["doc_id"])
        texts = _prepend_doc_summaries(texts, doc_ids)
    return texts


def _run_retrieval(question: str, top_k: int = None,
                   kb_ids: list | None = None,
                   document_ids: list | None = None,
                   include_expired: bool = False) -> list[dict]:
    """纯检索子管线（无规划器/LLM 生成）：实体检测 → 类1并行检索 → RRF → 精排 → 邻域补全。

    Agent 检索场景（/search）专用：查询已是 Agent 改写后的检索文本，
    不做规划器改写；不生成答案，返回精排后的 chunk 候选（含 text 与元数据）。
    kb_ids：KB 白名单（RRF 融合后过滤）；None/空 = 全库检索。
    document_ids：App 文档 UUID 白名单（融合后过滤）；None/空 = 不按文档过滤。
    """
    query_type = _detect_query_type(question)

    with ThreadPoolExecutor(max_workers=2) as executor:
        future_faiss = submit_with_trace(
            executor, faiss_searcher.search, question, FAISS_TOP_K
        )
        future_bm25 = submit_with_trace(
            executor, bm25_searcher.search, question, BM25_TOP_K,
        ) if (bm25_searcher and bm25_searcher.ready) else None

        faiss_hits = future_faiss.result()
        bm25_results = future_bm25.result() if future_bm25 else []

    faiss_results = _resolve_faiss_results(faiss_hits)
    merged = fusion.rrf_fusion(faiss_results, None, [], bm25_results)
    merged = _filter_by_kb(merged, kb_ids)
    merged = _expand_siblings(merged)
    merged = _attach_doc_meta(merged)
    merged = _filter_by_documents(merged, document_ids)
    reranked = fusion.rerank(
        question, merged, top_k=(top_k or FINAL_TOP_K) + 4, query_type=query_type,
    )
    reranked = _apply_lifecycle(question, reranked, include_expired)
    reranked = _append_neighbors(reranked, merged)
    if not include_expired:
        reranked = _drop_expired(reranked)
    return reranked


@app.route("/search", methods=["POST"])
def handle_search():
    """app(fusion) Agent 检索接口：纯检索无生成，供 knowledge_search 工具调用。

    参数（JSON）:
      queries:            检索文本数组（Agent 一次工具调用的多查询）
      knowledge_base_ids: KB 标识数组（白名单过滤；空 = 全库检索）
      document_ids:       App 文档 UUID 数组（白名单过滤；空 = 不按文档过滤）
      top_k:              返回条数（默认 FINAL_TOP_K）

    返回: {"results": [{chunk_id, document_id, file_name, content, score,
                        rerank_score, org_name, doc_type, product_name,
                        doc_number, doc_date, source}]}
    """
    body = request.get_json(silent=True) or {}
    queries = body.get("queries") or []
    if isinstance(queries, str):
        queries = [queries]
    queries = [q.strip() for q in queries if q and q.strip()]
    if not queries:
        return jsonify({"results": []})

    knowledge_base_ids = body.get("knowledge_base_ids") or []
    document_ids = body.get("document_ids") or []
    if not isinstance(document_ids, list):
        document_ids = []
    document_ids = [str(item).strip() for item in document_ids if str(item).strip()]
    include_expired = bool(body.get("include_expired"))
    try:
        top_k = int(body.get("top_k") or 0)
    except (TypeError, ValueError):
        top_k = 0
    if top_k <= 0:
        top_k = FINAL_TOP_K

    if knowledge_base_ids:
        logger.info(f"[search] knowledge_base_ids={knowledge_base_ids}（KB 过滤已启用）")
    if document_ids:
        logger.info(f"[search] document_ids={document_ids}（文档过滤已启用）")

    kb_ids = list(knowledge_base_ids) or None

    merged_by_id: dict[str, dict] = {}
    for q in queries:
        try:
            reranked = _run_retrieval(q, top_k=top_k, kb_ids=kb_ids,
                                      document_ids=document_ids,
                                      include_expired=include_expired)
        except Exception as e:
            logger.error(f"[search] 查询 {q[:60]} 检索异常: {e}", exc_info=True)
            continue
        for c in reranked:
            cid = c.get("chunk_id")
            if cid in (None, -1, "", "-1"):
                continue
            key = str(cid)
            prev = merged_by_id.get(key)
            if prev is None or (c.get("rerank_score") or 0) > (prev.get("rerank_score") or 0):
                merged_by_id[key] = c

    results = sorted(
        merged_by_id.values(),
        key=lambda c: -(c.get("rerank_score") or 0),
    )[:top_k]

    out = []
    for c in results:
        cid = c.get("chunk_id")
        out.append({
            "chunk_id": str(cid),
            "app_chunk_id": chunk_store.source_chunk_id(cid) if chunk_store else "",
            "document_id": str(c.get("document_id", "")),
            "source_doc_id": c.get("source_doc_id", "") or "",
            "file_name": c.get("file_name", ""),
            "content": c.get("text", ""),
            "score": c.get("score"),
            "rerank_score": c.get("rerank_score"),
            "org_name": c.get("org_name", ""),
            "doc_type": c.get("doc_type", ""),
            "product_name": c.get("product_name", ""),
            "doc_number": c.get("doc_number", ""),
            "doc_date": str(c.get("doc_date", "") or ""),
            "source": c.get("source", ""),
        })
    logger.info(f"[search] {len(queries)} 查询 → {len(out)} 条结果 (top_k={top_k})")
    return jsonify({"results": out})


def _filter_by_kb(candidates: list[dict], kb_ids) -> list[dict]:
    """按知识库白名单过滤融合后候选。kb_ids 为空（全库）不过滤。

    离线管线入库的存量文档 kb 为空会被滤掉——需挂到 KB 时手动 UPDATE。
    """
    if not kb_ids:
        return candidates
    kbs = set(kb_ids)
    kept = [
        c for c in candidates
        if c.get("knowledge_base_id") in kbs
    ]
    dropped = len(candidates) - len(kept)
    if dropped:
        logger.info(f"[KB过滤] kb={sorted(kbs)} 剔除 {dropped} 条候选，保留 {len(kept)} 条")
    return kept


def _filter_by_documents(candidates: list[dict], document_ids) -> list[dict]:
    """按 App 文档 UUID 白名单过滤融合后候选（精读选中文档）。

    source_doc_id 是 app 侧文档 UUID，由 _attach_doc_meta 从 documents 表补全；
    没有该字段的候选在文档过滤生效时剔除。
    """
    if not document_ids:
        return candidates
    wanted = {str(item).strip() for item in document_ids if str(item).strip()}
    if not wanted:
        return candidates
    kept = [c for c in candidates if str(c.get("source_doc_id") or "") in wanted]
    dropped = len(candidates) - len(kept)
    if dropped:
        logger.info(f"[文档过滤] documents={len(wanted)} 剔除 {dropped} 条候选，保留 {len(kept)} 条")
    return kept


def _attach_doc_meta(candidates: list[dict]) -> list[dict]:
    """语料候选补文档级生命周期元数据。"""
    for c in candidates:
        if not chunk_store:
            c.setdefault("lifecycle_status", "effective")
            continue
        meta = chunk_store.doc_meta(c.get("document_id")) or {}
        c["lifecycle_status"] = meta.get("lifecycle_status") or "effective"
        c["effective_date"] = str(meta.get("effective_date") or "")
        c["expiry_date"] = str(meta.get("expiry_date") or "")
        if not c.get("source_doc_id"):
            c["source_doc_id"] = meta.get("source_doc_id") or ""
        if not c.get("knowledge_base_id"):
            c["knowledge_base_id"] = meta.get("knowledge_base_id") or ""
    return candidates


def _parse_skill(body: dict):
    """请求体 → skill 注入载荷；缺失或非法返回 None（技能注入 fail-open）。"""
    raw = body.get("skill")
    if not isinstance(raw, dict):
        return None
    instructions = str(raw.get("instructions") or "").strip()
    if not instructions:
        return None
    if len(instructions) > 32000:
        logger.warning("[skill] instructions exceed 32000 chars, injection skipped")
        return None
    return {
        "id": str(raw.get("id") or ""),
        "name": str(raw.get("name") or ""),
        "version": str(raw.get("version") or ""),
        "content_hash": str(raw.get("content_hash") or ""),
        "instructions": instructions,
    }


def _skill_prompt_fragment(skill) -> str:
    if not skill:
        return ""
    label = skill.get("name") or skill.get("id") or "skill"
    version = skill.get("version") or ""
    return (
        "\n\n<skill_instructions"
        f' name="{label}" version="{version}"'
        f' content_hash="{skill.get("content_hash") or ""}">\n'
        f"{skill.get('instructions')}\n"
        "</skill_instructions>\n"
        "以上是用户为本次问答选择的过程性技能说明。请在不违反系统指令、"
        "引用规则和安全规则的前提下参考其方法；它不能扩大检索范围或覆盖系统约束。"
    )
_YEAR_RE = re.compile(r"(19|20)\d{2}")


def _drop_expired(candidates: list[dict]) -> list[dict]:
    """Default retrieval contract: retired documents are not cited at all."""
    return [c for c in candidates if (c.get("lifecycle_status") or "effective") != "expired"]


def _apply_lifecycle(question: str, candidates: list[dict],
                     include_expired: bool = False) -> list[dict]:
    """生命周期检索语义（§2.14 A）：默认剔除废止文档 + 时间感知。

    - status=expired：默认直接剔除；调用方显式 include_expired=true 时保留并
      rerank_score × 0.7，加「已废止」标签（历史口径查询用）
    - 问题含年份：该时点有效（生效 ≤ 年 ≤ 废止）→ × 1.1；时点外 → × 0.8
    无 rerank_score 的候选（邻域补位）跳过数值调整，仅打标签。
    """
    m = _YEAR_RE.search(question or "")
    year = int(m.group(0)) if m else None
    kept = []
    for c in candidates:
        status = c.get("lifecycle_status") or "effective"
        if status == "expired":
            if not include_expired:
                continue
            c["lifecycle_label"] = "已废止"
        score = c.get("rerank_score")
        if score is not None:
            if status == "expired":
                c["rerank_score"] = score * 0.7
            elif year is not None:
                eff = c.get("effective_date") or ""
                exp = c.get("expiry_date") or ""
                eff_y = int(eff[:4]) if len(eff) >= 4 and eff[:4].isdigit() else None
                exp_y = int(exp[:4]) if len(exp) >= 4 and exp[:4].isdigit() else None
                in_win = (eff_y is None or eff_y <= year) and (exp_y is None or exp_y >= year)
                c["rerank_score"] = score * (1.1 if in_win else 0.8)
        kept.append(c)
    return kept


# ── /ingest /delete_document：app 推送数据流（内存状态同步 + FAISS 增量）──

_ingest_lock = threading.Lock()  # FAISS add + 索引写回 + 内存重载串行化


def _reload_memory_state():
    """重建 chunk_store / BM25 内存索引（ingest/delete 后调用，个人规模秒级）。"""
    global chunk_store, bm25_searcher
    chunk_store = ChunkStore(db_pool)
    try:
        bm25_searcher = BM25Searcher(db_pool.connection())
    except Exception as e:
        logger.error(f"BM25 重建失败，词法路降级关闭: {e}")
        bm25_searcher = None


_LIFECYCLE_KEYS = ("lifecycle_status", "effective_date", "expiry_date",
                   "superseded_by")


def _lifecycle_fields(meta: dict) -> dict:
    """meta dict → 企业级元数据字段(§2.14 默认值:effective/internal)。

    旧格式键(status/effective_date/expiry_date)兼容:Go 侧新 payload 用
    lifecycle_status,离线管线/旧调用方可能仍用 status。
    """
    def _v(*keys, default=""):
        for k in keys:
            v = (meta or {}).get(k)
            if v not in (None, ""):
                return str(v).strip()
        return default
    return {
        "lifecycle_status": _v("lifecycle_status", "status", default="effective"),
        "effective_date": _v("effective_date"),
        "expiry_date": _v("expiry_date"),
        "superseded_by": _v("superseded_by"),
    }


def _ingest_markdown(kb_id: str, doc_uid: str, file_name: str,
                     file_type: str, markdown: str, meta: dict,
                     app_chunks: list = None) -> dict:
    """App 分块直接索引：删旧重建 → 写 chunks → 嵌入 → FAISS 追加 → 内存重载。

    RAG 不再解析或切分文档，app_chunks 是唯一内容来源；每块保留 App chunk id
    作为 source_chunk_id，供问答引用回查。
    """
    from ingest.pipeline import DB as IngestDB, call_bge_embedding

    ordered = sorted(
        [c for c in (app_chunks or [])
         if isinstance(c, dict) and str(c.get("content") or "").strip()],
        key=lambda c: int(c.get("chunk_index") or 0),
    )
    if not ordered:
        raise ValueError("chunks 为空：没有可索引的 App 分块")

    def _as_int(value, default: int = 0) -> int:
        try:
            return int(value)
        except (TypeError, ValueError):
            return default

    db = IngestDB()
    try:
        with db.conn.cursor() as cur:
            cur.execute("SELECT id FROM documents WHERE source_doc_id=%s", (doc_uid,))
            old = cur.fetchone()
        if old:
            with db.conn.cursor() as cur:
                cur.execute("DELETE FROM documents WHERE id=%s", (old[0],))
            db.commit()
            logger.info(f"[ingest] 文档 {doc_uid} 已存在(doc_id={old[0]})，删旧重建")

        life = _lifecycle_fields(meta)
        doc_id = db.insert_document(
            file_name=file_name, file_type=file_type,
            file_size=len(markdown.encode("utf-8")), file_path="",
            doc_number=meta.get("doc_number", ""), org_name=meta.get("org_name", ""),
            doc_date_str=meta.get("doc_date", ""),
            doc_title=meta.get("doc_title") or file_name,
            doc_type=meta.get("doc_type", ""), product_name=meta.get("product_name", ""),
            source_doc_id=doc_uid, knowledge_base_id=kb_id,
            lifecycle_status=life["lifecycle_status"],
            effective_date=life["effective_date"], expiry_date=life["expiry_date"],
            superseded_by=life["superseded_by"],
            custom_metadata=meta.get("custom_metadata"),
        )
        log_id = db.insert_parse_log(doc_id, "indexing", "running")

        chunk_ids = []
        for i, chunk in enumerate(ordered):
            chunk_type = str(chunk.get("chunk_type") or "text").strip().lower()
            chunk_ids.append(db.insert_chunk(
                document_id=doc_id,
                chunk_index=_as_int(chunk.get("chunk_index"), i),
                chunk_text=str(chunk["content"]),
                chunk_level="child",
                keywords="",
                pos_start=_as_int(chunk.get("start_at")),
                pos_end=_as_int(chunk.get("end_at")),
                org_name=meta.get("org_name", ""),
                doc_type=meta.get("doc_type", ""),
                product_name=meta.get("product_name", ""),
                knowledge_base_id=kb_id,
                lifecycle_status=life["lifecycle_status"],
                effective_date=life["effective_date"],
                expiry_date=life["expiry_date"],
                source_chunk_id=str(chunk.get("id") or ""),
                chunk_type=chunk_type,
                image_url=str(chunk.get("image_url") or ""),
                image_caption=str(chunk.get("image_caption") or ""),
                image_ocr=str(chunk.get("image_ocr") or ""),
                parent_source_chunk_id=str(chunk.get("parent_chunk_id") or ""),
            ))
        db.commit()
        logger.info(f"[ingest] doc_id={doc_id} 写入 {len(chunk_ids)} chunks")

        embedded = 0
        if faiss_searcher is not None and chunk_ids:
            texts = [str(c["content"]) for c in ordered]
            embs = call_bge_embedding(texts)
            if embs and len(embs) == len(texts):
                with _ingest_lock:
                    base = faiss_searcher.add_vectors_atomic(embs)
                    db.batch_update_vector_ids(
                        [(cid, base + i) for i, cid in enumerate(chunk_ids)]
                    )
                embedded = len(chunk_ids)
            else:
                logger.warning(
                    f"[ingest] doc_id={doc_id} 嵌入失败（{len(texts)} 块留 pending）"
                )

        db.update_document_status(doc_id, "success", chunk_count=len(chunk_ids))
        db.update_parse_log(log_id, "success", f"{len(chunk_ids)} chunks")
        db.insert_parse_log(doc_id, "done", "success", "入库完成")

        with _ingest_lock:
            _reload_memory_state()

        return {
            "status": "success", "document_id": doc_uid, "doc_id": doc_id,
            "chunks": len(chunk_ids), "embedded": embedded,
        }
    except Exception:
        db.rollback()
        raise
    finally:
        db.close()


@app.route("/ingest", methods=["POST"])
def handle_ingest():
    """app 数据流推送：markdown 文本或 xlsx 原始文件（base64）+ 元数据 → 入库。

    参数（JSON）:
      knowledge_base_id: app 侧 KB UUID（必填）
      document_id:       app 侧文档 UUID（必填，幂等键，重复推送=删旧重建）
      file_name / file_type: 必填
      markdown: 文本内容（常规文档必填）
      file_bytes: xlsx 原始文件 base64（file_type=xlsx 时可选，缺则回退 markdown）
      doc_number / org_name / doc_date / doc_title / doc_type / product_name: 可选元数据
    """
    body = request.get_json(silent=True) or {}
    kb_id = (body.get("knowledge_base_id") or "").strip()
    doc_uid = (body.get("document_id") or "").strip()
    file_name = (body.get("file_name") or "").strip()
    file_type = (body.get("file_type") or "").strip().lower() or "md"
    markdown = body.get("markdown") or ""
    if not kb_id or not doc_uid or not file_name:
        return jsonify({"error": "knowledge_base_id/document_id/file_name 必填"}), 400
    if not markdown.strip():
        return jsonify({"error": "markdown 必填"}), 400

    meta = {k: (body.get(k) or "").strip() for k in (
        "doc_number", "org_name", "doc_date", "doc_title", "doc_type", "product_name",
        "lifecycle_status", "effective_date", "expiry_date",
        "superseded_by",
    )}
    app_chunks = body.get("chunks") or []
    if not isinstance(app_chunks, list) or not app_chunks:
        return jsonify({"error": "chunks 必填：RAG 只索引 App 生成的分块"}), 400
    if isinstance(body.get("custom_metadata"), dict):
        meta["custom_metadata"] = body["custom_metadata"]

    logger.info(
        f"[ingest] kb={kb_id} doc={doc_uid} file={file_name} "
        f"markdown {len(markdown)} 字符"
    )
    try:
        result = _ingest_markdown(kb_id, doc_uid, file_name, file_type, markdown, meta, app_chunks)
        return jsonify(result)
    except Exception as e:
        logger.error(f"[ingest] 摄取异常 doc={doc_uid}: {e}", exc_info=True)
        return jsonify({"status": "failed", "document_id": doc_uid, "error": str(e)}), 500


@app.route("/delete_document", methods=["POST"])
def handle_delete_document():
    """app 文档删除同步：按 app 侧 document_id 删（MySQL 级联 + 内存镜像重载）。

    FAISS 为 HNSW 增量索引不支持删向量，残留由检索层过滤
    （vector_id → chunk 映射已清），物理清理靠 rebuild_faiss.py。
    """
    body = request.get_json(silent=True) or {}
    doc_uid = (body.get("document_id") or "").strip()
    if not doc_uid:
        return jsonify({"error": "document_id 必填"}), 400

    from ingest.pipeline import DB as IngestDB
    db = IngestDB()
    try:
        with db.conn.cursor() as cur:
            cur.execute("SELECT id FROM documents WHERE source_doc_id=%s", (doc_uid,))
            old = cur.fetchone()
        if not old:
            return jsonify({"deleted": 0, "document_id": doc_uid})
        with db.conn.cursor() as cur:
            cur.execute("DELETE FROM documents WHERE id=%s", (old[0],))
        db.commit()
        logger.info(f"[delete] doc={doc_uid} (doc_id={old[0]}) 已删除")
    finally:
        db.close()

    with _ingest_lock:
        _reload_memory_state()
    return jsonify({"deleted": 1, "document_id": doc_uid})


@app.route("/update_metadata", methods=["POST"])
def handle_update_metadata():
    """文档元数据原地更新：生命周期变化不重切分、不重嵌入。

    只改 documents 行并同步内存镜像，下一次查询立即按新状态过滤；
    文档不存在返回 updated=0，app 侧回退整篇重推。
    """
    body = request.get_json(silent=True) or {}
    doc_uid = str(body.get("document_id") or "").strip()
    if not doc_uid:
        return jsonify({"error": "document_id 必填"}), 400
    life = _lifecycle_fields(body)

    from ingest.pipeline import DB as IngestDB
    db = IngestDB()
    doc_id = None
    try:
        with db.conn.cursor() as cur:
            cur.execute("SELECT id FROM documents WHERE source_doc_id=%s", (doc_uid,))
            row = cur.fetchone()
            if not row:
                return jsonify({"updated": 0, "document_id": doc_uid})
            doc_id = row[0]
            cur.execute(
                "UPDATE documents SET lifecycle_status=%s, effective_date=%s, "
                "expiry_date=%s, superseded_by=%s "
                "WHERE id=%s",
                (
                    life["lifecycle_status"], life["effective_date"] or None,
                    life["expiry_date"] or None, life["superseded_by"] or None, doc_id,
                ),
            )
        db.commit()
        logger.info(
            f"[update_metadata] doc={doc_uid} (doc_id={doc_id}) "
            f"lifecycle={life['lifecycle_status']}"
        )
    finally:
        db.close()

    with _ingest_lock:
        if chunk_store is not None and chunk_store.doc_meta(doc_id):
            chunk_store.patch_doc_meta(
                doc_id,
                lifecycle_status=life["lifecycle_status"],
                effective_date=life["effective_date"],
                expiry_date=life["expiry_date"],
            )
        else:
            _reload_memory_state()
    return jsonify({"updated": 1, "document_id": doc_uid})


@app.route("/documents/ids", methods=["GET"])
def handle_document_ids():
    """对账用(§2.2 ③)：返回全部 source_doc_id（即 app 侧 document_id 镜像）。"""
    from ingest.pipeline import DB as IngestDB
    db = IngestDB()
    try:
        with db.conn.cursor() as cur:
            cur.execute("SELECT source_doc_id FROM documents")
            ids = [row[0] for row in cur.fetchall()]
    finally:
        db.close()
    return jsonify({"ids": ids})


@app.route("/index/status", methods=["GET"])
def handle_index_status():
    """索引健康（运维看板）：向量数、分块数、待嵌/失败、索引文件新鲜度。"""
    info = {
        "status": "ok", "vectors": 0, "documents": 0, "child_chunks": 0,
        "pending_chunks": 0, "failed_chunks": 0,
        "index_path": "", "file_size": 0, "file_updated_at": "",
        "previous_index": False,
    }
    if faiss_searcher is not None:
        info["vectors"] = int(faiss_searcher.index.ntotal)
        path = faiss_searcher.index_path
        info["index_path"] = os.path.basename(path)
        if os.path.exists(path):
            st = os.stat(path)
            info["file_size"] = st.st_size
            info["file_updated_at"] = time.strftime(
                "%Y-%m-%dT%H:%M:%S", time.localtime(st.st_mtime)
            )
            info["previous_index"] = os.path.exists(path + ".prev")
    try:
        conn = db_pool.connection()
        try:
            with conn.cursor() as cur:
                cur.execute("SELECT COUNT(*) FROM documents")
                info["documents"] = int(cur.fetchone()[0])
                cur.execute("SELECT COUNT(*) FROM chunks WHERE chunk_level='child'")
                info["child_chunks"] = int(cur.fetchone()[0])
                cur.execute(
                    "SELECT COUNT(*) FROM chunks WHERE chunk_level='child' AND status='pending'"
                )
                info["pending_chunks"] = int(cur.fetchone()[0])
                cur.execute(
                    "SELECT COUNT(*) FROM chunks WHERE chunk_level='child' AND status='failed'"
                )
                info["failed_chunks"] = int(cur.fetchone()[0])
        finally:
            conn.close()
    except Exception as exc:
        info["status"] = "degraded"
        info["error"] = str(exc)
    return jsonify(info)


@app.route("/health", methods=["GET"])
def health():
    return jsonify({"status": "ok"})


def _start_keepalive():
    """防网关 serverless 冷池再冷却（冷启动 40-60s，主办方 30s 作答硬约束下首问撞冷=必超时）。

    每 KEEPALIVE_INTERVAL 秒 ping 一次仍在走网关的端点（chat 恒网关；
    embedding/rerank 仅对应开关为 gateway 时），best-effort 失败静默。
    """
    from config import (
        DEEPSEEK_URL, DEEPSEEK_API_KEY, DEEPSEEK_MODEL,
        BGE_EMBEDDING_URL, BGE_RERANKER_URL, BGE_API_KEY,
        BGE_EMBED_BACKEND, BGE_RERANK_BACKEND, KEEPALIVE_INTERVAL,
    )
    from http_client import post_json

    if KEEPALIVE_INTERVAL <= 0:
        return

    h = {"Authorization": f"Bearer {BGE_API_KEY}", "Content-Type": "application/json"}
    targets = [
        ("chat", DEEPSEEK_URL,
         {"model": DEEPSEEK_MODEL,
          "messages": [{"role": "user", "content": "只回复OK"}],
          "max_tokens": 4, "thinking": {"type": "disabled"}}),
    ]
    if BGE_EMBED_BACKEND == "gateway":
        targets.append(("embedding", BGE_EMBEDDING_URL,
                        {"model": "BAAI/bge-m3", "input": ["预热"]}))
    if BGE_RERANK_BACKEND == "gateway":
        targets.append(("rerank", BGE_RERANKER_URL,
                        {"model": "BAAI/bge-reranker-v2-m3",
                         "query": "预热", "documents": ["预热"]}))

    def loop():
        while True:
            time.sleep(KEEPALIVE_INTERVAL)
            for name, url, payload in targets:
                try:
                    post_json(url, payload, h, 10, retries=1)
                except Exception:
                    pass

    threading.Thread(target=loop, daemon=True, name="gateway-keepalive").start()
    logger.info(
        f"[keepalive] 已启动（每 {KEEPALIVE_INTERVAL}s ping "
        f"{'/'.join(t[0] for t in targets)}）"
    )


def _start_reembed_daemon():
    """定时重嵌 pending chunks（嵌入失败自愈链）：每 REEMBED_INTERVAL_MINUTES 检查，
    有 pending 则走 _build_faiss_index 增量追加 + 内存镜像重建。
    与 /ingest 共用 _ingest_lock 串行化 FAISS 写；rebuild_faiss.py 留作手动工具。
    """
    from config import REEMBED_INTERVAL_MINUTES
    from ingest.pipeline import DB as IngestDB, _build_faiss_index

    if REEMBED_INTERVAL_MINUTES <= 0:
        logger.info("REEMBED_INTERVAL_MINUTES<=0，重嵌 daemon 关闭")
        return

    def loop():
        while True:
            time.sleep(REEMBED_INTERVAL_MINUTES * 60)
            try:
                db = IngestDB()
                try:
                    with db.conn.cursor() as cur:
                        cur.execute(
                            "SELECT COUNT(*) FROM chunks "
                            "WHERE chunk_level='child' AND status='pending'"
                        )
                        pending = cur.fetchone()[0]
                    if pending == 0:
                        continue
                    logger.info(f"[reembed] 发现 {pending} 个 pending chunks，开始重嵌")
                    with _ingest_lock:
                        _build_faiss_index(db)
                        _reload_memory_state()
                finally:
                    db.close()
            except Exception as e:
                logger.error(f"[reembed] daemon 异常: {e}", exc_info=True)

    threading.Thread(target=loop, daemon=True, name="reembed-daemon").start()
    logger.info(f"重嵌 daemon 已启动（每 {REEMBED_INTERVAL_MINUTES} 分钟检查一次）")


if __name__ == "__main__":
    init_services()
    _start_keepalive()
    _start_reembed_daemon()
    # 主办方固定问答端口经 PORT 环境变量传入（.env），默认 5000
    port = int(os.getenv("PORT", "5000"))
    app.run(host="0.0.0.0", port=port, debug=False)
