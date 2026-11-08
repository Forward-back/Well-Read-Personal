# BASE_IMAGE 由 compose build.args 传入镜像代理前缀（国内网络加速），海外网络可去掉
ARG BASE_IMAGE=python:3.12-slim
FROM ${BASE_IMAGE}

WORKDIR /app

# pip 源可覆盖；USTC 镜像的 numpy 等 wheel 哈希与 PyPI 一致
ARG PIP_INDEX_URL=https://pypi.mirrors.ustc.edu.cn/simple/

# Python 依赖（先装 faiss 等重依赖，利用 Docker 缓存）
COPY requirements.txt .
RUN pip install --no-cache-dir --retries 10 --timeout 120 -i ${PIP_INDEX_URL} -r requirements.txt

# 本地 BGE 模型推理依赖（BGE_EMBED_BACKEND / BGE_RERANK_BACKEND =local 时启用）。
# torch 走阿里云 CPU 轮子镜像（官方 download.pytorch.org 国内直连常挂死）；
# transformers 4.44.2：FlagEmbedding 1.3.3 的依赖上限（>4.45 会冲突）
RUN pip install --no-cache-dir --timeout 600 \
        "https://mirrors.aliyun.com/pytorch-wheels/cpu/torch-2.6.0%2Bcpu-cp312-cp312-linux_x86_64.whl" \
    && pip install --no-cache-dir --retries 10 --timeout 300 -i ${PIP_INDEX_URL} \
        transformers==4.44.2 FlagEmbedding==1.3.3 peft

# 代码
COPY src/ ./src/
COPY init.sql .
RUN mkdir -p /app/external/Data /app/external/models

# 入口：等待 MySQL → 按需 ingest → 按需重建索引 → 启动 API
COPY entrypoint.sh .
RUN chmod +x entrypoint.sh

EXPOSE 6151

ENTRYPOINT ["./entrypoint.sh"]
