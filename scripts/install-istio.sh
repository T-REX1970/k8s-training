#!/usr/bin/env bash
# =============================================================================
# Istio インストールスクリプト
# istioctl を使って Istio をminikubeに導入する
# Istioはマニフェストが複雑なため、istioctlによるインストールが公式推奨
# 実行: bash scripts/install-istio.sh
# =============================================================================

set -euo pipefail

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
CYAN='\033[0;36m'
NC='\033[0m'

info()    { echo -e "${GREEN}[INFO]${NC} $*"; }
warn()    { echo -e "${YELLOW}[WARN]${NC} $*"; }
error()   { echo -e "${RED}[ERROR]${NC} $*"; exit 1; }
section() { echo -e "\n${CYAN}=== $* ===${NC}"; }

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MANIFESTS_DIR="${SCRIPT_DIR}/../manifests"

# Istioバージョン
ISTIO_VERSION="1.20.2"

# -------------------------------------------
# 前提条件チェック
# -------------------------------------------
section "前提条件確認"

command -v kubectl >/dev/null 2>&1 || error "kubectlがインストールされていません"
kubectl cluster-info >/dev/null 2>&1 || error "Kubernetesクラスタに接続できません"

# minikubeが十分なリソースを持っているか確認
info "クラスタ情報:"
kubectl get nodes -o wide

# -------------------------------------------
# istioctlのインストール
# -------------------------------------------
section "istioctl インストール (v${ISTIO_VERSION})"

if command -v istioctl >/dev/null 2>&1; then
  CURRENT_VERSION=$(istioctl version --remote=false 2>/dev/null | head -1 || echo "unknown")
  info "istioctlは既にインストール済みです: ${CURRENT_VERSION}"
else
  info "istioctlをダウンロードしています..."

  # 公式インストールスクリプトを使用
  curl -L https://istio.io/downloadIstio | ISTIO_VERSION="${ISTIO_VERSION}" TARGET_ARCH=x86_64 sh -

  # PATHに追加
  export PATH="${PWD}/istio-${ISTIO_VERSION}/bin:${PATH}"

  info "istioctlをインストールしました"
  info "恒久的にPATHに追加するには以下を~/.bashrcや~/.zshrcに追記してください:"
  info "  export PATH=\"\${HOME}/istio-${ISTIO_VERSION}/bin:\${PATH}\""
fi

# -------------------------------------------
# Istio のインストール (demo profile)
# demo profile: 学習・検証向け (全機能有効、本番より軽量設定)
# minimal profile: コントロールプレーンのみ
# default profile: 本番向け標準設定
# -------------------------------------------
section "Istio インストール (demo profile)"

info "プロファイル: demo"
info "  - istiod (コントロールプレーン)"
info "  - istio-ingressgateway"
info "  - istio-egressgateway"
info "  - Kiali (サービスメッシュ可視化)"
info "  - Jaeger (分散トレーシング統合)"
info "  - Grafana (Istioダッシュボード)"
info "  - Prometheus (Istioメトリクス)"

istioctl install --set profile=demo -y

info "Istioインストール完了"

# -------------------------------------------
# インストール確認
# -------------------------------------------
section "Istio インストール確認"

info "istio-system Namespace のPodを確認..."
kubectl get pods -n istio-system

# -------------------------------------------
# Istio アドオン (Kiali, Jaeger, Grafana, Prometheus) インストール
# -------------------------------------------
section "Istio アドオンインストール"

ISTIO_DIR="${PWD}/istio-${ISTIO_VERSION}"
if [ -d "${ISTIO_DIR}/samples/addons" ]; then
  info "Kialiをインストール..."
  kubectl apply -f "${ISTIO_DIR}/samples/addons/kiali.yaml"

  info "Jaeger (Istio統合版) をインストール..."
  kubectl apply -f "${ISTIO_DIR}/samples/addons/jaeger.yaml"

  info "アドオンの反映を待機..."
  kubectl rollout status deployment/kiali -n istio-system --timeout=120s
else
  warn "Istioアドオンディレクトリが見つかりません: ${ISTIO_DIR}/samples/addons"
  warn "手動でistioctlダッシュボードを使用してください:"
  warn "  istioctl dashboard kiali"
fi

# -------------------------------------------
# monitoring Namespaceへのサイドカー自動インジェクション
# -------------------------------------------
section "サイドカー自動インジェクション設定"

info "monitoring NamespaceにIstioサイドカー注入ラベルを付与..."
kubectl label namespace monitoring istio-injection=enabled --overwrite

info "設定確認:"
kubectl get namespace monitoring --show-labels

warn "既存のPodにサイドカーを注入するには、Podを再起動してください:"
warn "  kubectl rollout restart deployment -n monitoring"

# -------------------------------------------
# サンプルIstioリソースの適用確認
# -------------------------------------------
section "Istioサンプルリソース"

info "サンプルCRD (Gateway/VirtualService/DestinationRule) の適用準備完了"
info "適用コマンド:"
echo "  kubectl apply -f ${MANIFESTS_DIR}/istio/sample-gateway.yaml"
echo "  kubectl apply -f ${MANIFESTS_DIR}/istio/sample-virtualservice.yaml"
echo "  kubectl apply -f ${MANIFESTS_DIR}/istio/sample-destinationrule.yaml"
echo "  kubectl apply -f ${MANIFESTS_DIR}/istio/peer-authentication.yaml"

# -------------------------------------------
# 完了メッセージ
# -------------------------------------------
section "インストール完了"

echo ""
info "各ダッシュボードへのアクセス方法:"
echo ""
echo "  # Kiali (サービスメッシュ可視化)"
echo "  istioctl dashboard kiali"
echo ""
echo "  # Jaeger (Istio統合版)"
echo "  istioctl dashboard jaeger"
echo ""
echo "  # Grafana (Istioダッシュボード)"
echo "  istioctl dashboard grafana"
echo ""
echo "  # Istio Ingress Gateway のURLを確認"
echo "  minikube service istio-ingressgateway -n istio-system"
echo ""
echo "次のステップ:"
echo "  1. monitoring Namespaceのポットを再起動してサイドカーを注入"
echo "     kubectl rollout restart deployment -n monitoring"
echo "  2. Kialiでサービスグラフを確認"
echo "  3. mTLS設定 (peer-authentication.yaml) を適用"
