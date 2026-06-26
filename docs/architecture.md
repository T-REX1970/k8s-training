# Kubernetes クラスター構成図

> 最終更新: 2026-02-28 / 環境: minikube (WSL2 + Docker)

---

## 全体アーキテクチャ

```mermaid
graph TB
    %% ===== 外部 =====
    subgraph External["🌐 外部アクセス (Windows ブラウザ)"]
        User["👤 ユーザー<br/>localhost経由でアクセス"]
        GitHub["🐙 GitHub<br/>k8s-training repo<br/>(GitOpsソース)"]
    end

    %% ===== WSL2 =====
    subgraph WSL2["🐧 WSL2 (Ubuntu 24.04) / kubectl port-forward"]

        %% ===== minikube クラスター =====
        subgraph Cluster["☸️ minikube Cluster  CPU:4core / MEM:8GB / K8s:v1.34.0"]

            %% istio-system namespace
            subgraph NS_ISTIO["📦 istio-system"]
                Istiod["istiod<br/>コントロールプレーン<br/>ClusterIP:15010/15012"]
                IGW["istio-ingressgateway<br/>NodePort:30080(HTTP)<br/>NodePort:30443(HTTPS)"]
                Kiali["Kiali v2.22<br/>サービスメッシュ可視化<br/>NodePort:20001"]
            end

            %% monitoring namespace
            subgraph NS_MONITORING["📦 monitoring  ※ Istio sidecar injection: enabled"]

                subgraph Metrics["📊 メトリクス収集・保存"]
                    Prometheus["Prometheus<br/>:30090"]
                    VM["VictoriaMetrics<br/>:30428 /vmui"]
                    AlertManager["AlertManager<br/>:30903"]
                    NodeExporter["node-exporter<br/>DaemonSet"]
                    KSM["kube-state-metrics"]
                end

                subgraph Logging["📋 ログ収集・保存"]
                    Loki["Loki v2.9<br/>ClusterIP:3100"]
                    Promtail["Promtail<br/>DaemonSet"]
                end

                subgraph Tracing["🔍 分散トレーシング"]
                    Jaeger["Jaeger v2<br/>All-in-One<br/>ClusterIP:16686"]
                end

                subgraph Viz["📈 可視化"]
                    Grafana["Grafana v10<br/>:30300<br/>admin/admin"]
                end
            end

            %% argocd namespace
            subgraph NS_ARGOCD["📦 argocd  (未インストール / Phase5)"]
                ArgoServer["argocd-server<br/>:30808"]
                ArgoCtrl["application-controller<br/>差分検知・同期"]
                ArgoRepo["repo-server<br/>Git取得"]
                ArgoRedis["redis / dex"]
            end

            %% apps namespace
            subgraph NS_APPS["📦 apps  (未作成 / Phase3)"]
                App["Sample App<br/>OpenTelemetry計装済み"]
                Sidecar["Envoy Sidecar<br/>Istio自動注入"]
            end

            %% kube-system
            subgraph NS_KUBE["⚙️ kube-system"]
                CoreDNS["CoreDNS"]
                KubeProxy["kube-proxy"]
            end

        end
    end

    %% ===== ユーザーアクセス (port-forward経由) =====
    User -->|"localhost:3000"| Grafana
    User -->|"localhost:9090"| Prometheus
    User -->|"localhost:8428"| VM
    User -->|"localhost:16686"| Jaeger
    User -->|"localhost:20001"| Kiali
    User -->|"localhost:8080 (未)"| ArgoServer

    %% ===== GitOps フロー =====
    GitHub -.->|"60秒ごとpoll (未)"| ArgoRepo
    ArgoRepo -.-> ArgoCtrl
    ArgoCtrl -.->|"自動同期 (未)"| NS_MONITORING

    %% ===== Istio サイドカー =====
    Istiod -->|"設定配布 xDS"| Sidecar
    IGW -->|"L7 ルーティング"| Sidecar
    Sidecar --> App
    Kiali -->|"メトリクス参照"| Prometheus

    %% ===== メトリクス収集フロー =====
    Prometheus -->|"remote_write"| VM
    Prometheus -->|"scrape"| NodeExporter
    Prometheus -->|"scrape"| KSM
    Prometheus -->|"scrape (未)"| App
    Prometheus -->|"alerts"| AlertManager

    %% ===== ログ収集フロー =====
    Promtail -->|"/var/log/pods 読取"| Loki

    %% ===== トレース収集フロー =====
    App -.->|"OTLP gRPC :4317 (未)"| Jaeger

    %% ===== Grafana クエリ =====
    Grafana -->|"PromQL"| Prometheus
    Grafana -->|"MetricsQL"| VM
    Grafana -->|"LogQL"| Loki
    Grafana -->|"TraceQL"| Jaeger

    %% ===== スタイル =====
    classDef running  fill:#d5f5e3,stroke:#27ae60,stroke-width:2px
    classDef pending  fill:#fef9e7,stroke:#f39c12,stroke-width:1px,stroke-dasharray:5 5
    classDef infra    fill:#eaf4fb,stroke:#2980b9,stroke-width:1px
    classDef mesh     fill:#f5eef8,stroke:#8e44ad,stroke-width:2px

    class Prometheus,VM,AlertManager,NodeExporter,KSM running
    class Loki,Promtail,Jaeger,Grafana running
    class Istiod,IGW,Kiali mesh
    class ArgoServer,ArgoCtrl,ArgoRepo,ArgoRedis pending
    class App,Sidecar pending
    class CoreDNS,KubeProxy infra
```

---

## Helm リリース一覧

```mermaid
graph LR
    subgraph Helm["🎡 Helm で管理されているリリース"]

        subgraph Mon["namespace: monitoring"]
            H1["vm<br/>vm/victoria-metrics-single<br/>v1.x"]
            H2["monitoring<br/>prometheus-community/kube-prometheus-stack<br/>Prometheus + Grafana + AlertManager"]
            H3["loki<br/>grafana/loki-stack<br/>Loki + Promtail ⚠️deprecated"]
            H4["jaeger<br/>jaegertracing/jaeger<br/>v2.15.1"]
        end

        subgraph Istio["namespace: istio-system"]
            H5["istio-base<br/>istio/base<br/>CRD群"]
            H6["istiod<br/>istio/istiod<br/>コントロールプレーン"]
            H7["istio-ingressgateway<br/>istio/gateway<br/>NodePort:30080/30443"]
            H8["kiali<br/>kiali/kiali-server<br/>v2.22.0"]
        end

        subgraph Argo["namespace: argocd (未インストール)"]
            H9["argocd<br/>argo/argo-cd<br/>予定"]
        end

    end
```

---

## データフロー詳細

```mermaid
sequenceDiagram
    participant Dev as 👤 開発者
    participant GH  as GitHub
    participant Argo as ArgoCD (未)
    participant K8s as Kubernetes API
    participant App as Sample App (未)
    participant Prom as Prometheus
    participant VM  as VictoriaMetrics
    participant Loki as Loki
    participant Jaeger as Jaeger
    participant Graf as Grafana

    Note over Dev,Graf: GitOps デプロイフロー (Phase5 以降)
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

## ポート一覧

### monitoring namespace

| サービス | Helm リリース | タイプ | ポート (port-forward) | 用途 |
|---|---|---|---|---|
| Grafana | monitoring | NodePort | **localhost:3000** | ダッシュボード (admin/admin) |
| Prometheus | monitoring | NodePort | **localhost:9090** | メトリクスUI・PromQL |
| VictoriaMetrics | vm | NodePort | **localhost:8428/vmui** | 高性能メトリクスDB・MetricsQL |
| AlertManager | monitoring | NodePort | **localhost:9093** | アラート管理UI |
| Jaeger UI | jaeger | ClusterIP | **localhost:16686** | 分散トレースUI |
| Loki | loki | ClusterIP | localhost:3100 | ログDB (Grafana経由で使用) |

### istio-system namespace

| サービス | Helm リリース | タイプ | ポート (port-forward) | 用途 |
|---|---|---|---|---|
| Kiali | kiali | NodePort | **localhost:20001** | サービスメッシュ可視化 |
| Ingress Gateway (HTTP) | istio-ingressgateway | NodePort | 30080 | 外部HTTPトラフィック |
| Ingress Gateway (HTTPS) | istio-ingressgateway | NodePort | 30443 | 外部HTTPSトラフィック |

### argocd namespace (未インストール)

| サービス | Helm リリース | タイプ | ポート | 用途 |
|---|---|---|---|---|
| ArgoCD Server | argocd | NodePort | localhost:8080 | GitOps UI |

---

## 学習フェーズ進捗

```mermaid
gantt
    title 学習フェーズとコンポーネント
    dateFormat  YYYY-MM-DD
    axisFormat  %m/%d

    section Phase1 環境構築
    minikube セットアップ       :done,    p1a, 2026-02-24, 3d
    Helm インストール            :done,    p1b, 2026-02-24, 3d

    section Phase2 Observability
    Prometheus + Grafana        :done,    p2a, 2026-02-28, 1d
    VictoriaMetrics             :done,    p2b, 2026-02-28, 1d
    Jaeger                      :done,    p2c, 2026-02-28, 1d
    Loki + Promtail             :done,    p2d, 2026-02-28, 1d

    section Phase4 Service Mesh
    Istio (Helm)                :done,    p4a, 2026-02-28, 1d
    Kiali                       :done,    p4b, 2026-02-28, 1d
    サンプルアプリ (mTLS確認)  :active,  p4c, 2026-03-01, 7d

    section Phase3 サンプルアプリ
    OTel計装アプリ作成          :         p3a, 2026-03-07, 7d
    メトリクス・トレース確認    :         p3b, 2026-03-07, 7d

    section Phase5 GitOps
    ArgoCD インストール         :         p5a, 2026-03-14, 3d
    Gitリポジトリ連携           :         p5b, 2026-03-14, 7d

    section Phase6 統合・最適化
    全体動作確認                :         p6a, 2026-03-21, 7d
    ドキュメント整備            :         p6b, 2026-03-21, 7d
```
