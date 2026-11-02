#!/bin/bash
set -e

# MySQL 连接信息全部来自环境变量（env_file 传入），主办方 MySQL 或本地容器均可
MYSQL_HOST="${MYSQL_HOST:-mysql}"
MYSQL_PORT="${MYSQL_PORT:-3306}"
MYSQL_USER="${MYSQL_USER:-root}"
MYSQL_PASSWORD="${MYSQL_PASSWORD:?MYSQL_PASSWORD is required}"
MYSQL_DATABASE="${MYSQL_DATABASE:-YLSW_Service_Table}"
export MYSQL_HOST MYSQL_PORT MYSQL_USER MYSQL_PASSWORD MYSQL_DATABASE

echo "===== Waiting for MySQL ($MYSQL_HOST:$MYSQL_PORT)... ====="
for i in $(seq 1 30); do
    if python -c "
import os, pymysql
pymysql.connect(host=os.environ['MYSQL_HOST'], port=int(os.environ['MYSQL_PORT']),
                user=os.environ['MYSQL_USER'], password=os.environ['MYSQL_PASSWORD'])
" 2>/dev/null; then
        echo "MySQL is ready."
        break
    fi
    echo "  Waiting... ($i/30)"
    sleep 2
done

# 建库（主办方 MySQL 可能是空库）+ 表结构幂等初始化
echo "===== Init database ====="
python - <<'PYEOF'
import os, re, pymysql
host = os.environ['MYSQL_HOST']; port = int(os.environ['MYSQL_PORT'])
user = os.environ['MYSQL_USER']; pwd = os.environ['MYSQL_PASSWORD']
db = os.environ['MYSQL_DATABASE']

# 1) 空库场景：先不带 database 连接，动态建库（库名来自 env，不写死在 SQL）
conn = pymysql.connect(host=host, port=port, user=user, password=pwd)
with conn.cursor() as cur:
    try:
        cur.execute(f"CREATE DATABASE IF NOT EXISTS `{db.replace(chr(96), chr(96)*2)}` "
                    f"CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci")
        conn.commit()
        print(f'database `{db}` ready.')
    except Exception as e:
        # 无建库权限时假定库已由主办方建好，继续建表
        print(f'CREATE DATABASE skipped: {e}')
conn.close()

# 2) 建表（幂等，可重复执行）
conn = pymysql.connect(host=host, port=port, user=user, password=pwd, database=db)
with open('/app/init.sql', encoding='utf-8') as f:
    sql = f.read()
sql = re.sub(r'--[^\n]*', '', sql)
stmts = [s.strip() for s in sql.split(';') if s.strip()]
with conn.cursor() as cur:
    for s in stmts:
        cur.execute(s)
conn.commit()
conn.close()
print(f'{len(stmts)} statements executed.')
PYEOF

# 数据目录只存放索引与模型；语料解析由 App 负责。
echo "===== Prepare external data directory ====="
python - <<'PYEOF'
import os
from src.config import DATA_DIR
index_dir = os.path.dirname(DATA_DIR)
os.makedirs(DATA_DIR, exist_ok=True)
os.makedirs(index_dir, exist_ok=True)
files = [name for name in os.listdir(DATA_DIR) if os.path.isfile(os.path.join(DATA_DIR, name))]
if files:
    print(f"RAG data ready: {len(files)} files in {DATA_DIR}")
else:
    print(f"RAG data directory is empty: {DATA_DIR}")
print(f"FAISS index directory: {index_dir}")
PYEOF

# 空库首启引导：FAISS 索引文件缺失时建空 HNSW 索引（app 启动要求文件存在；
# 向量随 /ingest 增量追加）。有文件则跳过，不覆盖。
echo "===== Bootstrap empty FAISS index if missing ====="
python - <<'PYEOF'
import os
from src.config import DATA_DIR, EMBEDDING_DIM, HNSW_M
index_path = os.path.join(os.path.dirname(DATA_DIR), "faiss_index.bin")
if os.path.exists(index_path):
    print(f"FAISS index exists ({os.path.getsize(index_path)} bytes), skip bootstrap.")
else:
    import faiss, tempfile, shutil
    index = faiss.IndexHNSWFlat(EMBEDDING_DIM, HNSW_M)
    index.hnsw.efConstruction = 200
    # faiss C++ 不支持中文路径：写临时文件再 copy2 落位（与 ingest 侧同款规避）
    tmp = tempfile.NamedTemporaryFile(delete=False, suffix=".bin")
    tmp.close()
    try:
        faiss.write_index(index, tmp.name)
        shutil.copy2(tmp.name, index_path)
    finally:
        os.unlink(tmp.name)
    print(f"Empty FAISS index created: {index_path} (d={EMBEDDING_DIM}, ntotal=0)")
PYEOF

# 解析与分块由 App 完成；RAG 启动只负责数据库/索引就绪，不再扫描语料目录。

# 预热：本地开关对应的模型加载进内存（避免首问付加载代价）；
# gateway 开关对应的网关 serverless 实例提前拉起（冷启动 40-60s）。
# 失败不阻断启动，评分前由健康检查兜底。
echo "===== Warming up models (best-effort) ====="
python - <<'PYEOF'
import os
from src.config import BGE_EMBED_BACKEND, BGE_RERANK_BACKEND

# 本地模型预热（仅对应开关为 local 时加载）
if BGE_EMBED_BACKEND == "local" or BGE_RERANK_BACKEND == "local":
    from src.models_singleton import encode_texts, rerank_scores
    for name, backend, fn in [
        ("embedding", BGE_EMBED_BACKEND, lambda: encode_texts(["预热"], 768)),
        ("reranker", BGE_RERANK_BACKEND, lambda: rerank_scores("预热", ["预热"])),
    ]:
        if backend != "local":
            continue
        try:
            fn()
            print(f"  warmup {name} (local): OK")
        except Exception as e:
            print(f"  warmup {name} (local): skipped ({e})")

# 网关预热（chat 恒走网关；embedding/rerank 仅对应开关为 gateway 时）
import requests
from src.config import (
    BGE_EMBEDDING_URL, BGE_API_KEY, BGE_RERANKER_URL,
    DEEPSEEK_URL, DEEPSEEK_API_KEY, DEEPSEEK_MODEL,
)
h = {"Authorization": f"Bearer {BGE_API_KEY}", "Content-Type": "application/json"}
gw_items = [
    ("chat", DEEPSEEK_URL, {"model": DEEPSEEK_MODEL,
        "messages": [{"role": "user", "content": "只回复OK"}],
        "max_tokens": 4, "thinking": {"type": "disabled"}}),
]
if BGE_EMBED_BACKEND == "gateway":
    gw_items.append(("embedding", BGE_EMBEDDING_URL,
        {"model": "BAAI/bge-m3", "input": ["预热"]}))
if BGE_RERANK_BACKEND == "gateway":
    gw_items.append(("rerank", BGE_RERANKER_URL,
        {"model": "BAAI/bge-reranker-v2-m3", "query": "预热", "documents": ["预热"]}))
for name, url, payload in gw_items:
    try:
        r = requests.post(url, json=payload, headers=h, timeout=120)
        print(f"  warmup {name}: HTTP {r.status_code}")
    except Exception as e:
        print(f"  warmup {name}: skipped ({e})")
PYEOF

echo "===== Starting API server ====="
exec python -m src.app
