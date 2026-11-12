# Cross-service scripts

只放跨 app / frontend / rag 的构建与运维脚本。

- `build_images.sh`：构建 gaze-app / gaze-docreader / gaze-ui 镜像。
- `package-lite.sh`：构建并打包 Lite 发行包。
- `check-env.sh`：检查 `deployments/.env` 与本机依赖。
- `ingest_data.py`：把本地语料批量导入 app 并触发 RAG reload；默认读取仓库内 `data/rag/Data`（gitignore）。
- `acceptance/rag_study_review.py`：对运行中的栈跑 10 题学习/复习验收；题库与断言在 `acceptance/rag_study_review_cases.json`。
- `git-hooks/`、`install-git-hooks.sh`：仓库级 git hooks。

服务专属脚本留在各自目录：Go 构建/迁移在 `app/scripts/`，前端构建在 `frontend/scripts/`，RAG 运维在 `rag/scripts/`。
