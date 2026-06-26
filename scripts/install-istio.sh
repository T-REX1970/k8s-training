#!/usr/bin/env bash
# =============================================================================
# Istio インストールスクリプト (Helm版)
#
# インストールするコンポーネント:
#   1. istio-base      (CRD / クラスタ共通リソース)
#   2. istiod          (コントロールプレーン)
#   3. istio-ingressgateway (Ingress Gateway)
#   4. Kiali           (サービスメッシュ可視化)
#
# 前提条件:
#   - minikube が起動済み
#   - helm コマンドが使用可能
#   - observabilityスタック導入済み (Prometheus/Grafana/Jaeger)
#
# 実行方法:
#   bash scripts/install-istio.sh
#
# アンインストール:
#   bash scripts/install-istio.sh --uninstall
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
MANIFESTS_DIR="${SCRIPT_DIR}/../manifests"
NAMESPACE="istio-system"

# ============================================================
# アンインストールモード
# ============================================================
if [[ "${1:-}" == "--uninstall" ]]; then
  section "Istio アンインストール"
  warn "以下のHelmリリースを削除します:"
  warn "  kiali / istio-ingressgateway / istiod / istio-base (namespace: ${NAMESPACE})"
  read -rp "続行しますか? [y/N]: " confirm
  [[ "${confirm}" =~ ^[Yy]$ ]] || { info "キャンセルしました"; exit 0; }

  helm uninstall kiali              -n "${NAMESPACE}" 2>/dev/null || warn "kiali は未インストール"
  helm uninstall istio-ingressgateway -n "${NAMESPACE}" 2>/dev/null || warn "istio-ingressgateway は未インストール"
  helm uninstall istiod             -n "${NAMESPACE}" 2>/dev/null || warn "istiod は未インストール"
  helm uninstall istio-base         -n "${NAMESPACE}" 2>/dev/null || warn "istio-base は未インストール"
  kubectl delete namespace "${NAMESPACE}" --ignore-not-found

  # monitoring Namespaceのサイドカー注入ラベルを削除
  kubectl label namespace monitoring istio-injection- 2>/dev/null || true

  info "アンインストール完了"
  exit 0
fi

# ============================================================
# 前提条件チェック
# ============================================================
section "前提条件確認"

command -v kubectl >/dev/null 2>&1 || error "kubectl がインストールされていません"
command -v helm    >/dev/null 2>&1 || error "helm がインストールされていません"

kubectl cluster-info >/dev/null 2>&1 || error "Kubernetesクラスタに接続できません。minikubeを起動してください"

CONTEXT=$(kubectl config current-context)
info "クラスタコンテキスト: ${CONTEXT}"
kubectl get nodes -o wide

# ============================================================
# Helm リポジトリ追加・更新
# ============================================================
section "Helm リポジトリ設定"

helm repo add istio https://istio-release.storage.googleapis.com/charts 2>/dev/null || true
helm repo add kiali https://kiali.org/helm-charts                        2>/dev/null || true
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
# Step 1: istio-base (CRD + クラスタ共通リソース)
# ============================================================
section "Step 1/4: istio-base インストール"

info "リリース名: istio-base  |  チャート: istio/base"
info "含まれるもの: Istio CRD (VirtualService, DestinationRule, Gateway など)"

helm upgrade --install istio-base istio/base \
  --namespace "${NAMESPACE}" \
  --wait \
  --timeout 5m

info "istio-base インストール完了"

# ============================================================
# Step 2: istiod (コントロールプレーン)
# ============================================================
section "Step 2/4: istiod インストール"

info "リリース名: istiod  |  チャート: istio/istiod"
info "含まれるもの: Pilot (設定配布) / サイドカー自動注入 Webhook"

helm upgrade --install istiod istio/istiod \
  --namespace "${NAMESPACE}" \
  --values "${VALUES_DIR}/istiod-values.yaml" \
  --wait \
  --timeout 10m

info "istiod インストール完了"
info "  インストール確認: istioctl verify-install"

# ============================================================
# Step 3: istio-ingressgateway
# ============================================================
section "Step 3/4: istio-ingressgateway インストール"

info "リリース名: istio-ingressgateway  |  チャート: istio/gateway"
info "NodePort: HTTP=30080 / HTTPS=30443"

helm upgrade --install istio-ingressgateway istio/gateway \
  --namespace "${NAMESPACE}" \
  --values "${VALUES_DIR}/istio-gateway-values.yaml" \
  --wait \
  --timeout 5m

info "istio-ingressgateway インストール完了"

# ============================================================
# Step 4: Kiali (サービスメッシュ可視化)
# ============================================================
section "Step 4/4: Kiali インストール"

info "リリース名: kiali  |  チャート: kiali/kiali-server"
info "含まれるもの: サービスグラフ / トラフィック可視化 / mTLS状態確認"

helm upgrade --install kiali kiali/kiali-server \
  --namespace "${NAMESPACE}" \
  --values "${VALUES_DIR}/kiali-values.yaml" \
  --wait \
  --timeout 5m

info "Kiali インストール完了"

# ============================================================
# monitoring Namespace へのサイドカー自動注入設定
# ============================================================
section "サイドカー自動注入設定"

info "monitoring Namespace に Istio サイドカー注入ラベルを付与..."
kubectl label namespace monitoring istio-injection=enabled --overwrite

info "設定確認:"
kubectl get namespace monitoring --show-labels

warn "既存Podにサイドカーを注入するには再起動が必要です:"
warn "  kubectl rollout restart deployment -n monitoring"

# ============================================================
# Istioサンプルリソースの適用案内
# ============================================================
section "Istio サンプルリソース"

info "以下のサンプルCRDが適用可能です (istiod起動後):"
echo "  kubectl apply -f ${MANIFESTS_DIR}/istio/istio-namespace-label.yaml"
echo "  kubectl apply -f ${MANIFESTS_DIR}/istio/sample-gateway.yaml"
echo "  kubectl apply -f ${MANIFESTS_DIR}/istio/sample-virtualservice.yaml"
echo "  kubectl apply -f ${MANIFESTS_DIR}/istio/sample-destinationrule.yaml"
echo "  kubectl apply -f ${MANIFESTS_DIR}/istio/peer-authentication.yaml"

# ============================================================
# 全体の起動確認
# ============================================================
section "起動確認"

info "istio-system Namespace の Pod 状態:"
kubectl get pods -n "${NAMESPACE}" -o wide

echo ""
info "istio-system Namespace の Service 状態:"
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
echo -e "${CYAN}Kiali${NC} (サービスメッシュ可視化)"
echo "  NodePort : http://${MINIKUBE_IP}:$(kubectl get svc kiali -n ${NAMESPACE} -o jsonpath='{.spec.ports[0].nodePort}' 2>/dev/null || echo '20001')"
echo "  pfwd     : kubectl port-forward svc/kiali -n ${NAMESPACE} 20001:20001"
echo "             → http://localhost:20001"
echo ""
echo -e "${CYAN}Istio Ingress Gateway${NC}"
echo "  HTTP  NodePort : http://${MINIKUBE_IP}:30080"
echo "  HTTPS NodePort : https://${MINIKUBE_IP}:30443"
echo ""
echo "──────────────────────────────────────────────────────"
echo ""
echo -e "${CYAN}Helm リリース確認${NC}"
echo "  helm list -n ${NAMESPACE}"
echo ""
echo -e "${CYAN}アンインストール${NC}"
echo "  bash scripts/install-istio.sh --uninstall"
echo ""
echo -e "${CYAN}次のステップ${NC}"
echo "  ArgoCDのインストール:"
echo "    bash scripts/install-argocd.sh"
