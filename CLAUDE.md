# Kubernetes Observability 学習プロジェクト

## プロジェクト概要

このプロジェクトは、minikube環境でKubernetes、Observability、Service Mesh、GitOpsを実践的に学習するためのものです。

**目的:**
- Kubernetes運用の実践スキル習得
- Prometheus/Grafana/Jaegerによる可観測性の実装
- VictoriaMetricsによる高性能メトリクス収集・長期保存の実装
- Istio Service Meshによるトラフィック管理
- ArgoCDによるGitOps実践
- SREエンジニアとしてのスキルセット構築

**学習者:**
- ネットワークエンジニア(ネットワークスペシャリスト試験学習中)
- AWS、Kubernetes、Docker実務経験あり
- Go、Rust学習中
- FinOps実践経験あり

## 技術スタック

### 環境
- **ローカル環境**: macOS + Docker Desktop + minikube
- **Kubernetesバージョン**: 1.28+
- **リソース**: CPU 4コア、メモリ 8GB

### コンポーネント
- **コンテナオーケストレーション**: Kubernetes (minikube)
- **メトリクス収集**: Prometheus
- **高性能メトリクスDB**: VictoriaMetrics (Prometheusの代替/補完)
- **可視化**: Grafana
- **分散トレーシング**: Jaeger
- **テレメトリ**: OpenTelemetry
- **Service Mesh**: Istio
- **GitOps**: ArgoCD
- **パッケージ管理**: Helm

## プロジェクト構造
```
~/k8s-observability-project/
├── .claude/
│   ├── claude.md              # このファイル
│   ├── skills/                # Claude Code Skills
│   │   ├── k8s-manifest-generator/
│   │   ├── k8s-servicemonitor-generator/
│   │   └── promql-generator/
│   └── agents/                # (将来的に)Sub-agents
├── manifests/                 # Kubernetesマニフェスト
│   ├── prometheus/
│   ├── victoria-metrics/
│   ├── grafana/
│   ├── jaeger/
│   ├── istio/
│   └── apps/
├── terraform/                 # (将来的に)Terraformコード
├── docs/                      # ドキュメント
│   ├── setup.md
│   ├── troubleshooting.md
│   └── learning-notes.md
└── scripts/                   # セットアップスクリプト
    ├── setup-minikube.sh
    └── install-observability.sh
```

## 開発規約

### Kubernetesマニフェスト作成ルール

#### 必須項目
1. **セキュリティ**
   - SecurityContextを必ず設定
   - runAsNonRoot: true
   - readOnlyRootFilesystem: true (可能な限り)
   - capabilities.drop: [ALL]

2. **リソース管理**
   - resources.requests と resources.limits を必ず設定
   - 推奨値: CPU 100m-200m, Memory 128Mi-256Mi (開発環境)

3. **可用性**
   - livenessProbe と readinessProbe を必ず設定
   - レプリカ数: 本番想定は3以上、開発環境は1-2

4. **ラベル**
   - app.kubernetes.io/* 形式の推奨ラベルを使用
```yaml
   labels:
     app.kubernetes.io/name: <app-name>
     app.kubernetes.io/instance: <instance-name>
     app.kubernetes.io/version: "1.0.0"
     app.kubernetes.io/component: <component>
     app.kubernetes.io/part-of: <system>
     app.kubernetes.io/managed-by: argocd
```

5. **イメージ**
   - `latest` タグは禁止
   - 具体的なバージョンタグを指定 (例: nginx:1.25.3)

#### ファイル命名規則
- `<app-name>-<resource-type>.yaml`
- 例: `my-app-deployment.yaml`, `my-app-service.yaml`

### PromQLクエリ作成ルール

1. **時間範囲**
   - 短期トレンド: [5m]
   - アラート用: [1m]
   - ダッシュボード: [5m] または [15m]

2. **必ずコメントを付ける**
```promql
   # HTTPリクエストのP95レイテンシ (過去5分間)
   histogram_quantile(0.95,
     sum(rate(http_request_duration_seconds_bucket[5m])) by (le)
   )
```

3. **Golden Signalsを意識**
   - Latency (レイテンシ)
   - Traffic (トラフィック量)
   - Errors (エラー率)
   - Saturation (リソース飽和度)

4. **VictoriaMetrics MetricsQL**
   - PromQL互換だが拡張機能を活用する
   - `rate()` より `irate()` を使うケースを意識する
   - `rollup_*` 系関数はVMQL固有なので注釈を付ける

### コードスタイル

#### YAML
- インデント: 2スペース
- 日本語コメントを積極的に使用
- リソース定義の順序: metadata → spec → (その他)

#### Go (OpenTelemetry計装)
- gofmt準拠
- エラーハンドリングを必ず実装
- コンテキスト伝播を正しく行う

## よく使うコマンド

### minikube
```bash
# 起動
minikube start --driver=docker --cpus=4 --memory=8192

# 停止
minikube stop

# 削除
minikube delete

# ダッシュボード
minikube dashboard

# サービスアクセス
minikube service <service-name>
```

### kubectl
```bash
# リソース一覧
kubectl get all -A

# Pod詳細
kubectl describe pod <pod-name>

# ログ確認
kubectl logs <pod-name> -f

# 実行中のコンテナに入る
kubectl exec -it <pod-name> -- /bin/bash

# ポートフォワード
kubectl port-forward svc/<service-name> <local-port>:<remote-port>
```

### Helm
```bash
# リポジトリ追加
helm repo add <name> <url>
helm repo update

# インストール
helm install <release-name> <chart> --namespace <ns> --create-namespace

# アンインストール
helm uninstall <release-name> -n <namespace>

# 値の確認
helm get values <release-name> -n <namespace>
```

### VictoriaMetrics
```bash
# vmctlによるデータインポート (Prometheusからの移行)
vmctl prometheus --prom-snapshot=/path/to/snapshot --vm-addr=http://victoria-metrics:8428

# ヘルスチェック
curl http://victoria-metrics:8428/health

# メトリクス確認
curl http://victoria-metrics:8428/metrics

# クラスタ構成 (vminsert/vmselect/vmstorage)
# vminsert: 書き込みエンドポイント (:8480)
# vmselect: 読み取りエンドポイント (:8481)
# vmstorage: ストレージ (:8482)
```

## 学習フェーズ

### Phase 1: 環境構築 (Week 1)
- [ ] minikube セットアップ
- [ ] Claude Code Skills 作成 (3つ)
- [ ] 基本的なKubernetesリソースデプロイ確認

### Phase 2: Observability基盤 (Week 2-3)
- [ ] Prometheus + Grafana インストール
- [ ] Jaeger インストール
- [ ] OpenTelemetry Collector 設定
- [ ] 基本ダッシュボード作成
- [ ] アラートルール設定

### Phase 2.5: VictoriaMetrics (Week 3-4)
- [ ] VictoriaMetrics Single インストール (Prometheus代替として試す)
- [ ] Prometheus Remote Write → VictoriaMetrics 連携
- [ ] Grafana データソースとして VictoriaMetrics 追加
- [ ] vmagent でメトリクス収集
- [ ] vmalert でアラートルール設定
- [ ] VictoriaMetrics Cluster 構成を試す (vminsert/vmselect/vmstorage)
- [ ] Prometheusとのパフォーマンス・リソース比較

### Phase 3: サンプルアプリ (Week 4-5)
- [ ] OpenTelemetry計装アプリ作成
- [ ] ServiceMonitor 設定
- [ ] メトリクス・トレース収集確認

### Phase 4: Service Mesh (Week 5-6)
- [ ] Istio インストール
- [ ] カナリアデプロイ実装
- [ ] サーキットブレーカー設定
- [ ] mTLS 有効化
- [ ] Kiali でトラフィック可視化

### Phase 5: GitOps (Week 7)
- [ ] ArgoCD インストール
- [ ] Gitリポジトリ連携
- [ ] 自動デプロイ確認

### Phase 6: 統合と最適化 (Week 8)
- [ ] 全体の動作確認
- [ ] パフォーマンスチューニング
- [ ] ドキュメント整備

## Claude Codeへの指示

### マニフェスト作成時
- 必ず開発規約に従ったマニフェストを生成してください
- SecurityContext、Probes、resourcesは必須です
- 日本語コメントを付けてください

### トラブルシューティング時
- まず症状を理解し、考えられる原因を列挙してください
- デバッグコマンドを提示してください
- ネットワーク層の問題も考慮してください(学習者はネットワークエンジニア)

### PromQL / MetricsQL作成時
- Golden Signalsを意識してください
- 必ず日本語コメントを付けてください
- 時間範囲は[5m]を基本としてください
- VictoriaMetrics固有のMetricsQL関数を使う場合はその旨を明記してください

### VictoriaMetrics関連
- SingleモードとClusterモードの違いを意識してください
- vmagent、vminsert、vmselect、vmstorageの役割を説明してください
- Prometheusとの比較(メモリ使用量・ストレージ効率)を示してください

### ドキュメント作成時
- Markdown形式で作成してください
- コードブロックには適切な言語指定をしてください
- 初心者にも分かりやすい説明を心がけてください

## トラブルシューティング

### よくある問題

#### minikubeが起動しない
```bash
# Docker Desktopが起動しているか確認
docker ps

# minikubeを再起動
minikube stop
minikube start
```

#### Podが起動しない (ImagePullBackOff)
```bash
# イベント確認
kubectl describe pod <pod-name>

# イメージ名、タグを確認
```

#### Podが起動しない (CrashLoopBackOff)
```bash
# ログ確認
kubectl logs <pod-name> --previous

# リソース不足の可能性
kubectl describe node
```

#### VictoriaMetrics が起動しない
```bash
# ログ確認
kubectl logs -n monitoring <victoria-metrics-pod>

# ストレージのパーミッション確認 (runAsNonRootの場合)
kubectl describe pod <victoria-metrics-pod> -n monitoring

# データディレクトリの確認
kubectl exec -it <victoria-metrics-pod> -n monitoring -- ls -la /victoria-metrics-data
```

## 参考リソース

### 公式ドキュメント
- Kubernetes: https://kubernetes.io/docs/
- Prometheus: https://prometheus.io/docs/
- VictoriaMetrics: https://docs.victoriametrics.com/
- Grafana: https://grafana.com/docs/
- Jaeger: https://www.jaegertracing.io/docs/
- Istio: https://istio.io/latest/docs/
- ArgoCD: https://argo-cd.readthedocs.io/

### 学習リソース
- 『Kubernetes完全ガイド 第2版』
- 『詳解 システム・パフォーマンス』
- 『データ指向アプリケーションデザイン』
- Google SRE Book (無料公開)

## 連絡先・メモ

学習の進捗や気づきをここに記録してください。

---

**Last Updated**: 2026-02-28
**Current Phase**: Phase 1 - 環境構築
**Next Milestone**: Claude Code Skills作成完了
