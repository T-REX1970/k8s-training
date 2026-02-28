#!/usr/bin/env bash
# =============================================================================
# Observabilityスタック インストールスクリプト
# 対象: Prometheus, VictoriaMetrics, Grafana, Jaeger, Loki, Promtail
# 実行: bash scripts/install-observability.sh
# =============================================================================

set -euo pipefail

# カラー出力用
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
CYAN='\033[0;36m'
NC='\033[0m'

info()    { echo -e "${GREEN}[INFO]${NC} $*"; }
warn()    { echo -e "${YELLOW}[WARN]${NC} $*"; }
error()   { echo -e "${RED}[ERROR]${NC} $*"; exit 1; }
section() { echo -e "\n${CYAN}=== $* ===${NC}"; }

# スクリプトの場所を基準にパスを解決
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MANIFESTS_DIR="${SCRIPT_DIR}/../manifests"

# -------------------------------------------
# 前提条件チェック
# -------------------------------------------
section "前提条件確認"

command -v kubectl >/dev/null 2>&1 || error "kubectlがインストールされていません"

# minikubeが動いているか確認
kubectl cluster-info >/dev/null 2>&1 || error "Kubernetesクラスタに接続できません。minikubeを起動してください"

info "クラスタ接続OK"

# -------------------------------------------
# Namespace作成
# -------------------------------------------
section "Namespace作成"

kubectl apply -f "${MANIFESTS_DIR}/namespace.yaml"
info "monitoring Namespace作成完了"

# -------------------------------------------
# Prometheus インストール
# -------------------------------------------
section "Prometheus インストール"

info "RBAC設定を適用..."
kubectl apply -f "${MANIFESTS_DIR}/prometheus/prometheus-serviceaccount.yaml"
kubectl apply -f "${MANIFESTS_DIR}/prometheus/prometheus-clusterrole.yaml"
kubectl apply -f "${MANIFESTS_DIR}/prometheus/prometheus-clusterrolebinding.yaml"

info "ConfigMap適用..."
kubectl apply -f "${MANIFESTS_DIR}/prometheus/prometheus-configmap.yaml"

info "PVC作成..."
kubectl apply -f "${MANIFESTS_DIR}/prometheus/prometheus-pvc.yaml"

info "Deployment・Service適用..."
kubectl apply -f "${MANIFESTS_DIR}/prometheus/prometheus-deployment.yaml"
kubectl apply -f "${MANIFESTS_DIR}/prometheus/prometheus-service.yaml"

info "Prometheusインストール完了"

# -------------------------------------------
# VictoriaMetrics インストール
# -------------------------------------------
section "VictoriaMetrics インストール"

kubectl apply -f "${MANIFESTS_DIR}/victoria-metrics/victoria-metrics-pvc.yaml"
kubectl apply -f "${MANIFESTS_DIR}/victoria-metrics/victoria-metrics-deployment.yaml"
kubectl apply -f "${MANIFESTS_DIR}/victoria-metrics/victoria-metrics-service.yaml"

info "VictoriaMetricsインストール完了"

# -------------------------------------------
# Grafana インストール
# -------------------------------------------
section "Grafana インストール"

kubectl apply -f "${MANIFESTS_DIR}/grafana/grafana-configmap.yaml"
kubectl apply -f "${MANIFESTS_DIR}/grafana/grafana-pvc.yaml"
kubectl apply -f "${MANIFESTS_DIR}/grafana/grafana-deployment.yaml"
kubectl apply -f "${MANIFESTS_DIR}/grafana/grafana-service.yaml"

info "Grafanaインストール完了"

# -------------------------------------------
# Jaeger インストール
# -------------------------------------------
section "Jaeger インストール"

kubectl apply -f "${MANIFESTS_DIR}/jaeger/jaeger-deployment.yaml"
kubectl apply -f "${MANIFESTS_DIR}/jaeger/jaeger-service.yaml"

info "Jaegerインストール完了"

# -------------------------------------------
# Loki + Promtail インストール
# -------------------------------------------
section "Loki + Promtail インストール"

kubectl apply -f "${MANIFESTS_DIR}/loki/loki-configmap.yaml"
kubectl apply -f "${MANIFESTS_DIR}/loki/loki-pvc.yaml"
kubectl apply -f "${MANIFESTS_DIR}/loki/loki-deployment.yaml"
kubectl apply -f "${MANIFESTS_DIR}/loki/loki-service.yaml"

info "Promtail (ログコレクター) を適用..."
kubectl apply -f "${MANIFESTS_DIR}/loki/promtail-serviceaccount.yaml"
kubectl apply -f "${MANIFESTS_DIR}/loki/promtail-clusterrole.yaml"
kubectl apply -f "${MANIFESTS_DIR}/loki/promtail-clusterrolebinding.yaml"
kubectl apply -f "${MANIFESTS_DIR}/loki/promtail-configmap.yaml"
kubectl apply -f "${MANIFESTS_DIR}/loki/promtail-daemonset.yaml"

info "Loki + Promtailインストール完了"

# -------------------------------------------
# Podの起動待機
# -------------------------------------------
section "Pod起動待機"

info "全Deploymentが起動するまで待機します (最大5分)..."

kubectl wait --for=condition=available \
  deployment/prometheus \
  deployment/victoria-metrics \
  deployment/grafana \
  deployment/jaeger \
  deployment/loki \
  -n monitoring \
  --timeout=300s

info "DaemonSet (Promtail) の起動を確認..."
kubectl rollout status daemonset/promtail -n monitoring --timeout=120s

info "全Pod起動確認完了"

# -------------------------------------------
# アクセス情報の表示
# -------------------------------------------
section "インストール完了"

echo ""
kubectl get pods -n monitoring
echo ""
kubectl get svc -n monitoring
echo ""

info "=============================="
info "Observabilityスタック起動完了!"
info "=============================="
echo ""
echo "各サービスへのアクセス方法:"
echo ""
echo "  # Prometheus UI (NodePort 30090)"
echo "  minikube service prometheus -n monitoring"
echo ""
echo "  # VictoriaMetrics UI (NodePort 30428)"
echo "  minikube service victoria-metrics -n monitoring"
echo ""
echo "  # Grafana UI (NodePort 30300)"
echo "  minikube service grafana -n monitoring"
echo "  ログイン: admin / admin"
echo "  データソース: Prometheus / VictoriaMetrics / Loki"
echo ""
echo "  # Jaeger UI (NodePort 30686)"
echo "  minikube service jaeger -n monitoring"
echo ""
echo "または port-forward でアクセス:"
echo "  kubectl port-forward svc/prometheus -n monitoring 9090:9090"
echo "  kubectl port-forward svc/victoria-metrics -n monitoring 8428:8428"
echo "  kubectl port-forward svc/grafana -n monitoring 3000:3000"
echo "  kubectl port-forward svc/jaeger -n monitoring 16686:16686"
echo "  kubectl port-forward svc/loki -n monitoring 3100:3100"
echo ""
echo "Istio インストール (別途実行):"
echo "  bash scripts/install-istio.sh"
echo ""
echo "全リソース削除:"
echo "  kubectl delete -k ${MANIFESTS_DIR}/"
