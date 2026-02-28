# Kubernetes クラスター構成図

## 全体アーキテクチャ

```mermaid
graph TB
    %% ===== 外部 =====
    subgraph External["🌐 外部"]
        User["👤 ユーザー<br/>(ブラウザ / kubectl)"]
        GitHub["GitHub<br/>k8s-training repo"]
    end

    %% ===== minikube クラスター =====
    subgraph Cluster["☸️ minikube Cluster (CPU:4core / MEM:8GB)"]

        %% argocd namespace
        subgraph NS_ARGOCD["📦 namespace: argocd"]
            ArgoServer["argocd-server<br/>NodePort:30808"]
            ArgoRepo["argocd-repo-server<br/>(Git取得・マニフェスト生成)"]
            ArgoCtrl["argocd-application-controller<br/>(差分検知・同期)"]
            ArgoRedis["argocd-redis<br/>(キャッシュ)"]
            ArgoDex["argocd-dex-server<br/>(OIDC認証)"]
        end

        %% monitoring namespace
        subgraph NS_MONITORING["📦 namespace: monitoring"]

            subgraph Metrics["📊 メトリクス"]
                Prometheus["Prometheus<br/>NodePort:30090"]
                VM["VictoriaMetrics<br/>NodePort:30428"]
                AlertManager["AlertManager<br/>NodePort:30903"]
                NodeExporter["node-exporter<br/>(DaemonSet)"]
                KSM["kube-state-metrics"]
            end

            subgraph Logging["📋 ログ"]
                Loki["Loki<br/>ClusterIP:3100"]
                Promtail["Promtail<br/>(DaemonSet)"]
            end

            subgraph Tracing["🔍 トレーシング"]
                Jaeger["Jaeger All-in-One<br/>NodePort:30686"]
            end

            subgraph Visualization["📈 可視化"]
                Grafana["Grafana<br/>NodePort:30300"]
            end
        end

        %% istio-system namespace (Phase4)
        subgraph NS_ISTIO["📦 namespace: istio-system (Phase4)"]
            Istiod["istiod<br/>(コントロールプレーン)"]
            IGW["istio-ingressgateway"]
        end

        %% apps namespace (Phase3)
        subgraph NS_APPS["📦 namespace: apps (Phase3)"]
            App["Sample App<br/>(OpenTelemetry計装済み)"]
            Sidecar["Envoy Sidecar<br/>(Istio注入)"]
        end

    end

    %% ===== ユーザーアクセス =====
    User -->|"NodePort 30808"| ArgoServer
    User -->|"NodePort 30300"| Grafana
    User -->|"NodePort 30090"| Prometheus
    User -->|"NodePort 30428<br/>/vmui"| VM
    User -->|"NodePort 30686"| Jaeger

    %% ===== GitOps フロー =====
    GitHub -->|"60秒ごとにpoll"| ArgoRepo
    ArgoRepo --> ArgoCtrl
    ArgoCtrl -->|"kubectl apply<br/>(自動同期)"| NS_MONITORING

    %% ===== メトリクス収集フロー =====
    Prometheus -->|"remote_write"| VM
    Prometheus -->|"scrape /metrics"| NodeExporter
    Prometheus -->|"scrape /metrics"| KSM
    Prometheus -->|"scrape /metrics"| App
    Prometheus -->|"alerts"| AlertManager

    %% ===== ログ収集フロー =====
    Promtail -->|"/var/log/pods を読取"| Loki

    %% ===== トレース収集フロー =====
    App -->|"OTLP gRPC :4317"| Jaeger

    %% ===== Grafana クエリ =====
    Grafana -->|"PromQL"| Prometheus
    Grafana -->|"MetricsQL"| VM
    Grafana -->|"LogQL"| Loki
    Grafana -->|"TraceQL"| Jaeger

    %% ===== Istio (Phase4) =====
    IGW -->|"トラフィック制御"| Sidecar
    Sidecar --> App
    Istiod -->|"設定配布"| Sidecar

    %% ===== スタイル =====
    classDef namespace fill:#e8f4f8,stroke:#2980b9,stroke-width:2px
    classDef component fill:#ffffff,stroke:#7f8c8d,stroke-width:1px
    classDef gitops fill:#fef9e7,stroke:#f39c12,stroke-width:2px
    classDef future fill:#f8f9fa,stroke:#bdc3c7,stroke-width:1px,stroke-dasharray:5 5

    class ArgoServer,ArgoRepo,ArgoCtrl,ArgoRedis,ArgoDex gitops
    class Istiod,IGW,Sidecar future
    class App future
```

---

## データフロー詳細

```mermaid
sequenceDiagram
    participant Dev as 👤 開発者
    participant GH as GitHub
    participant Argo as ArgoCD
    participant K8s as Kubernetes API
    participant App as Sample App
    participant Prom as Prometheus
    participant VM as VictoriaMetrics
    participant Loki as Loki
    participant Jaeger as Jaeger
    participant Graf as Grafana

    Note over Dev,Graf: GitOps デプロイフロー
    Dev->>GH: git push (manifest変更)
    GH-->>Argo: 差分検知 (60秒以内)
    Argo->>K8s: kubectl apply (自動同期)
    K8s-->>App: Pod起動・更新

    Note over App,Graf: Observability データフロー
    App->>Prom: /metrics エンドポイント公開
    Prom->>App: スクレイプ (15秒ごと)
    Prom->>VM: remote_write 転送
    App->>Jaeger: OTLP でトレース送信
    App-->>Loki: ログ出力 → Promtailが収集

    Note over Dev,Graf: 可視化・アラート
    Dev->>Graf: ダッシュボード確認
    Graf->>Prom: PromQL クエリ
    Graf->>VM: MetricsQL クエリ
    Graf->>Loki: LogQL クエリ
    Graf->>Jaeger: トレース取得
    Prom-->>Dev: アラート発火 (AlertManager)
```

---

## Namespace 構成

```mermaid
graph LR
    subgraph Cluster["minikube Cluster"]
        subgraph A["argocd"]
            a1["argocd-server\n(NodePort 30808)"]
            a2["argocd-repo-server"]
            a3["argocd-application-controller"]
            a4["argocd-redis"]
            a5["argocd-dex-server"]
        end

        subgraph M["monitoring"]
            m1["Prometheus\n(NodePort 30090)"]
            m2["VictoriaMetrics\n(NodePort 30428)"]
            m3["Grafana\n(NodePort 30300)"]
            m4["AlertManager\n(NodePort 30903)"]
            m5["Loki\n(ClusterIP 3100)"]
            m6["Promtail\n(DaemonSet)"]
            m7["Jaeger\n(NodePort 30686)"]
            m8["node-exporter\n(DaemonSet)"]
            m9["kube-state-metrics"]
        end

        subgraph I["istio-system (Phase4)"]
            i1["istiod"]
            i2["istio-ingressgateway"]
        end

        subgraph AP["apps (Phase3)"]
            ap1["sample-app\n(OTel計装)"]
        end
    end
```

---

## ポート一覧

| サービス | Namespace | タイプ | ポート | 用途 |
|---|---|---|---|---|
| Grafana | monitoring | NodePort | **30300** | UI・ダッシュボード |
| Prometheus | monitoring | NodePort | **30090** | UI・クエリ |
| VictoriaMetrics | monitoring | NodePort | **30428** | UI (/vmui)・クエリ |
| AlertManager | monitoring | NodePort | **30903** | アラート管理UI |
| Jaeger | monitoring | NodePort | **30686** | トレースUI |
| Loki | monitoring | ClusterIP | 3100 | ログ書込・クエリ(内部のみ) |
| ArgoCD | argocd | NodePort | **30808** | GitOps UI |

---

## 学習フェーズとコンポーネントの対応

```mermaid
gantt
    title 学習フェーズとコンポーネント
    dateFormat  YYYY-MM-DD
    axisFormat  Week%W

    section Phase1 環境構築
    minikube セットアップ      :done, 2026-02-24, 7d
    Claude Code Skills作成     :done, 2026-02-24, 7d

    section Phase2 Observability基盤
    Prometheus + Grafana       :active, 2026-03-03, 7d
    Jaeger                     :active, 2026-03-03, 7d
    OpenTelemetry Collector    :2026-03-03, 7d

    section Phase2.5 VictoriaMetrics
    VM Single インストール     :2026-03-10, 7d
    Prometheus比較学習         :2026-03-10, 7d

    section Phase3 サンプルアプリ
    OTel計装アプリ作成         :2026-03-17, 7d
    メトリクス・トレース収集   :2026-03-17, 7d

    section Phase4 Service Mesh
    Istio インストール         :2026-03-24, 14d
    mTLS・カナリアデプロイ     :2026-03-24, 14d

    section Phase5 GitOps
    ArgoCD インストール        :2026-04-07, 7d
    自動デプロイ確認           :2026-04-07, 7d

    section Phase6 統合・最適化
    全体動作確認               :2026-04-14, 7d
    ドキュメント整備           :2026-04-14, 7d
```
