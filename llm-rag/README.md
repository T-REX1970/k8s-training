# ローカルLLM RAGマイクロサービス学習プロジェクト

SREとしてPrometheus / Grafana / Jaeger / Istio / ArgoCD / Langfuse / LLM-as-judgeを
実践的に学習するための題材。RAGチャット自体の機能はあくまで手段。

実装ロードマップ・アーキテクチャの詳細は実装ブリーフを参照（Phase 0〜7）。

## 構成 (Phase 0)

```
client -> gateway-api -> chat-service -> llm-service -> Ollama
```

- `gateway-api`: `POST /api/chat`をchat-serviceにリバースプロキシ。レート制限、request-id付与。
- `chat-service`: セッション単位の会話履歴をメモリ保持し、プロンプトを組み立ててllm-serviceを呼ぶ（RAGはPhase 2で追加）。
- `llm-service`: Ollamaの`/api/generate`を呼び出す（モデル名は`OLLAMA_MODEL`環境変数で切替可能、Phase 5のカナリアで利用）。

各サービス共通:
- `GET /healthz`: liveness
- `GET /readyz`: readiness（下流サービス疎通確認込み）
- `log/slog`によるJSON構造化ログ（`request_id`をヘッダ`X-Request-Id`で伝搬）
- `SIGINT`/`SIGTERM`によるグレースフルシャットダウン

## モジュール構成

`go.work`でワークスペースを構成し、サービスごとに独立した`go.mod`を持つ
（実運用のマイクロサービスと同様に独立ビルド・独立バージョニングが可能）。

```
llm-rag/
├── go.work
└── services/
    ├── gateway-api/
    ├── chat-service/
    └── llm-service/
```

## ローカル動作確認 (docker-compose)

前提: Docker Desktop。WSL環境では Settings > Resources > WSL Integration で
対象ディストリビューションを有効化し、`docker compose`コマンドが使えることを確認してください。

```bash
cd llm-rag
docker compose -f docker-compose.dev.yaml up -d --build
```

初回はOllamaにモデルをpullする必要があります（`llama3.2:1b`は約1.3GB、CPUでも動く小型モデル）。

```bash
docker compose -f docker-compose.dev.yaml exec ollama ollama pull llama3.2:1b
```

### ヘルスチェック

```bash
curl http://localhost:8080/healthz   # gateway-api
curl http://localhost:8080/readyz    # -> chat-service疎通確認
curl http://localhost:8081/readyz    # chat-service -> llm-service疎通確認
curl http://localhost:8082/readyz    # llm-service -> Ollama疎通確認
```

### チャット (RAGなし、最小フロー)

```bash
curl -s -X POST http://localhost:8080/api/chat \
  -H 'Content-Type: application/json' \
  -d '{"message": "こんにちは、自己紹介してください"}' | jq
```

レスポンス例:
```json
{"session_id": "...", "response": "..."}
```

2回目以降は`session_id`を指定すると会話履歴を引き継ぐ（プロセス再起動で消える点に注意、永続化はPhase 3以降）。

```bash
curl -s -X POST http://localhost:8080/api/chat \
  -H 'Content-Type: application/json' \
  -d '{"session_id": "<上で返ってきたID>", "message": "さっきの内容を一言で要約して"}' | jq
```

### ログの確認

```bash
docker compose -f docker-compose.dev.yaml logs -f gateway-api chat-service llm-service
```

各行はJSON1行で、`request_id`を見れば3サービスを横断してリクエストを追跡できる
（Phase 1でJaegerによる分散トレーシングに置き換える前段の確認として有用）。

## 既知の制約 (Phase 0時点)

- ストリーミング応答は未実装（Ollamaからは`stream:false`で一括取得）。Phase 1以降のトレース計装と合わせて見直す。
- セッション履歴はメモリ上のみで永続化なし（Redis導入はPhase 3）。
- RAG検索（retrieval-service / embedding-service / Qdrant）は未実装（Phase 2）。
- レート制限はクライアント単位ではなく全体共有の単一トークンバケット。

## 次のフェーズ

Phase 1: Prometheus / Grafana / Jaeger導入。3サービス分の`/metrics`公開とトレース計装を追加する。
