#!/usr/bin/env bash
# =============================================================================
# Helm版 Observabilityスタック インストールスクリプト (minikube用)
#
# インストールするコンポーネント:
#   1. VictoriaMetrics Single     (高性能メトリクスDB)
#   2. kube-prometheus-stack      (Prometheus + Grafana + AlertManager +
#                                  node-exporter + kube-state-metrics)
#   3. Loki Stack                 (Loki + Promtail)
#   4. Jaeger                     (分散トレーシング)
#
# 前提条件:
#   - minikube が起動済み (bash scripts/setup-minikube.sh で起動)
#   - helm コマンドが使用可能
#
# 実行方法:
#   bash scripts/install-helm-observability.sh
#
# アンインストール:
#   bash scripts/install-helm-observability.sh --uninstall
# =============================================================================

set -euo pipefail

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

info()    { echo -e "${GREEN}[INFO]${NC} $*"; }
warn()    { echo -e "${YELLOW}[WARN]${NC} $*"; }
error()   { echo -e "${RED}[ERROR]${NC} $*"; exit 1; }
section() { echo -e "\n${CYAN}${BOLD}=== $* ===${NC}"; }

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
VALUES_DIR="${SCRIPT_DIR}/../helm/values"
NAMESPACE="monitoring"

# ============================================================
# アンインストールモード
# ============================================================
if [[ "${1:-}" == "--uninstall" ]]; then
  section "アンインストール"
  warn "以下のHelmリリースを削除します:"
  warn "  jaeger / loki / vm / monitoring (namespace: ${NAMESPACE})"
  read -rp "続行しますか? [y/N]: " confirm
  [[ "${confirm}" =~ ^[Yy]$ ]] || { info "キャンセルしました"; exit 0; }

  helm uninstall jaeger     -n "${NAMESPACE}" 2>/dev/null || warn "jaeger は未インストール"
  helm uninstall loki       -n "${NAMESPACE}" 2>/dev/null || warn "loki は未インストール"
  helm uninstall vm         -n "${NAMESPACE}" 2>/dev/null || warn "vm は未インストール"
  helm uninstall monitoring -n "${NAMESPACE}" 2>/dev/null || warn "monitoring は未インストール"
  kubectl delete namespace "${NAMESPACE}" --ignore-not-found

  info "アンインストール完了"
  exit 0
fi

# ============================================================
# 前提条件チェック
# ============================================================
section "前提条件確認"

command -v kubectl >/dev/null 2>&1 || error "kubectl がインストールされていません"
command -v helm    >/dev/null 2>&1 || error "helm がインストールされていません (brew install helm)"

kubectl cluster-info >/dev/null 2>&1 || error "Kubernetesクラスタに接続できません。minikubeを起動してください"

CONTEXT=$(kubectl config current-context)
info "クラスタコンテキスト: ${CONTEXT}"
kubectl get nodes -o wide

# ============================================================
# Helm リポジトリ追加・更新
# ============================================================
section "Helm リポジトリ設定"

helm repo add prometheus-community https://prometheus-community.github.io/helm-charts 2>/dev/null || true
helm repo add grafana              https://grafana.github.io/helm-charts              2>/dev/null || true
helm repo add vm                   https://victoriametrics.github.io/helm-charts      2>/dev/null || true
helm repo add jaegertracing        https://jaegertracing.github.io/helm-charts        2>/dev/null || true
helm repo update

info "リポジトリ登録済み:"
helm repo list

# ============================================================
# Namespace 作成
# ============================================================
section "Namespace 作成"

kubectl create namespace "${NAMESPACE}" --dry-run=client -o yaml | kubectl apply -f -
info "Namespace '${NAMESPACE}' 準備完了"

# ============================================================
# Step 1: VictoriaMetrics をインストール
# Prometheusのremote_writeより先に起動しておく
# ============================================================
section "Step 1/4: VictoriaMetrics インストール"

info "リリース名: vm  |  チャート: vm/victoria-metrics-single"
helm upgrade --install vm vm/victoria-metrics-single \
  --namespace "${NAMESPACE}" \
  --values "${VALUES_DIR}/victoria-metrics-values.yaml" \
  --wait \
  --timeout 5m

info "VictoriaMetrics インストール完了"
info "  サービス: vm-victoria-metrics-single-server.${NAMESPACE}:8428"
info "  NodePort: http://$(minikube ip):30428/vmui"

# ============================================================
# Step 2: kube-prometheus-stack をインストール
# Grafana に VM・Loki・Jaeger のデータソースを設定済み
# ============================================================
section "Step 2/4: kube-prometheus-stack インストール"

info "リリース名: monitoring  |  チャート: prometheus-community/kube-prometheus-stack"
info "含まれるもの: Prometheus / Grafana / AlertManager / node-exporter / kube-state-metrics"

helm upgrade --install monitoring prometheus-community/kube-prometheus-stack \
  --namespace "${NAMESPACE}" \
  --values "${VALUES_DIR}/kube-prometheus-stack-values.yaml" \
  --wait \
  --timeout 10m

info "kube-prometheus-stack インストール完了"

# ============================================================
# Step 3: Loki Stack をインストール (Loki + Promtail)
# ============================================================
section "Step 3/4: Loki Stack インストール"

info "リリース名: loki  |  チャート: grafana/loki-stack"
info "含まれるもの: Loki (ログDB) / Promtail (DaemonSet ログコレクター)"

helm upgrade --install loki grafana/loki-stack \
  --namespace "${NAMESPACE}" \
  --values "${VALUES_DIR}/loki-stack-values.yaml" \
  --wait \
  --timeout 5m

info "Loki Stack インストール完了"
info "  Lokiサービス: loki.${NAMESPACE}:3100"
info "  Promtail: 全ノードで起動中 (DaemonSet)"

# ============================================================
# Step 4: Jaeger をインストール (All-in-One)
# ============================================================
section "Step 4/4: Jaeger インストール"

info "リリース名: jaeger  |  チャート: jaegertracing/jaeger"
info "モード: All-in-One (Collector + Query + UI が1Pod)"

helm upgrade --install jaeger jaegertracing/jaeger \
  --namespace "${NAMESPACE}" \
  --values "${VALUES_DIR}/jaeger-values.yaml" \
  --wait \
  --timeout 5m

# Jaeger Query UIをNodePortで公開 (Helmの値でカバーできない場合の補完)
kubectl patch service jaeger-query \
  -n "${NAMESPACE}" \
  --type='json' \
  -p='[{"op":"replace","path":"/spec/type","value":"NodePort"},{"op":"add","path":"/spec/ports/0/nodePort","value":30686}]' \
  2>/dev/null || warn "jaeger-query NodePort 設定はすでに適用済みか手動設定が必要です"

info "Jaeger インストール完了"

# ============================================================
# 全体の起動確認
# ============================================================
section "起動確認"

info "全Pod の状態:"
kubectl get pods -n "${NAMESPACE}" -o wide

echo ""
info "全Service の状態:"
kubectl get svc -n "${NAMESPACE}"

# ============================================================
# アクセス情報
# ============================================================
MINIKUBE_IP=$(minikube ip 2>/dev/null || echo "127.0.0.1")

section "インストール完了"

echo ""
echo -e "${BOLD}各サービスへのアクセス方法${NC}"
echo "──────────────────────────────────────────────────────"
echo ""
echo -e "${CYAN}Grafana${NC} (ログイン: admin / admin)"
echo "  NodePort : http://${MINIKUBE_IP}:30300"
echo "  minikube : minikube service monitoring-grafana -n ${NAMESPACE}"
echo "  pfwd     : kubectl port-forward svc/monitoring-grafana -n ${NAMESPACE} 3000:80"
echo ""
echo -e "${CYAN}Prometheus${NC}"
echo "  NodePort : http://${MINIKUBE_IP}:30090"
echo "  pfwd     : kubectl port-forward svc/monitoring-kube-prometheus-prometheus -n ${NAMESPACE} 9090:9090"
echo ""
echo -e "${CYAN}AlertManager${NC}"
echo "  NodePort : http://${MINIKUBE_IP}:30903"
echo "  pfwd     : kubectl port-forward svc/monitoring-kube-prometheus-alertmanager -n ${NAMESPACE} 9093:9093"
echo ""
echo -e "${CYAN}VictoriaMetrics${NC} (vmUI 内蔵)"
echo "  NodePort : http://${MINIKUBE_IP}:30428/vmui"
echo "  pfwd     : kubectl port-forward svc/vm-victoria-metrics-single-server -n ${NAMESPACE} 8428:8428"
echo ""
echo -e "${CYAN}Loki${NC} (ClusterIP のみ / Grafana から参照)"
echo "  pfwd     : kubectl port-forward svc/loki -n ${NAMESPACE} 3100:3100"
echo ""
echo -e "${CYAN}Jaeger UI${NC}"
echo "  NodePort : http://${MINIKUBE_IP}:30686"
echo "  pfwd     : kubectl port-forward svc/jaeger-query -n ${NAMESPACE} 16686:16686"
echo ""
echo "──────────────────────────────────────────────────────"
echo ""
echo -e "${CYAN}Grafanaで確認できるデータソース${NC}"
echo "  - Prometheus      (デフォルト / PromQL)"
echo "  - VictoriaMetrics (PromQL + MetricsQL)"
echo "  - Loki            (LogQL)"
echo "  - Jaeger          (分散トレーシング)"
echo ""
echo -e "${CYAN}次のステップ${NC}"
echo "  Istio のインストール:"
echo "    bash scripts/install-istio.sh"
echo ""
echo -e "${CYAN}Helmリリースの確認${NC}"
echo "  helm list -n ${NAMESPACE}"
echo ""
echo -e "${CYAN}アンインストール${NC}"
echo "  bash scripts/install-helm-observability.sh --uninstall"
