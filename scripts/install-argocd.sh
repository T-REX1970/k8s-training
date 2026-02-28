#!/usr/bin/env bash
# =============================================================================
# ArgoCD インストールスクリプト (minikube用)
#
# インストール方法: Helm (argo/argo-cd チャート)
# Namespace: argocd
#
# 実行方法:
#   bash scripts/install-argocd.sh
#
# アンインストール:
#   bash scripts/install-argocd.sh --uninstall
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
NAMESPACE="argocd"

# ============================================================
# アンインストールモード
# ============================================================
if [[ "${1:-}" == "--uninstall" ]]; then
  section "ArgoCD アンインストール"
  warn "ArgoCD を削除します (namespace: ${NAMESPACE})"
  read -rp "続行しますか? [y/N]: " confirm
  [[ "${confirm}" =~ ^[Yy]$ ]] || { info "キャンセルしました"; exit 0; }

  helm uninstall argocd -n "${NAMESPACE}" 2>/dev/null || warn "argocd は未インストール"
  kubectl delete namespace "${NAMESPACE}" --ignore-not-found

  info "アンインストール完了"
  exit 0
fi

# ============================================================
# 前提条件チェック
# ============================================================
section "前提条件確認"

command -v kubectl >/dev/null 2>&1 || error "kubectl がインストールされていません"
command -v helm    >/dev/null 2>&1 || error "helm がインストールされていません"

kubectl cluster-info >/dev/null 2>&1 || \
  error "Kubernetesクラスタに接続できません。minikubeを起動してください"

CONTEXT=$(kubectl config current-context)
info "クラスタコンテキスト: ${CONTEXT}"

# ============================================================
# Helm リポジトリ追加
# ============================================================
section "Helm リポジトリ設定"

helm repo add argo https://argoproj.github.io/argo-helm 2>/dev/null || true
helm repo update
info "argo リポジトリ追加完了"

# ============================================================
# Namespace 作成
# ============================================================
section "Namespace 作成"

kubectl apply -f "${MANIFESTS_DIR}/argocd/argocd-namespace.yaml"
info "Namespace '${NAMESPACE}' 準備完了"

# ============================================================
# ArgoCD インストール
# ============================================================
section "ArgoCD インストール"

info "リリース名: argocd  |  チャート: argo/argo-cd"
info "含まれるもの:"
info "  - argocd-server          (UI / API)"
info "  - argocd-repo-server     (Git取得・マニフェスト生成)"
info "  - argocd-application-controller (差分検知・同期)"
info "  - argocd-applicationset-controller"
info "  - argocd-dex-server      (OIDC認証)"
info "  - argocd-redis           (キャッシュ)"
info "  - argocd-notifications-controller"

helm upgrade --install argocd argo/argo-cd \
  --namespace "${NAMESPACE}" \
  --values "${VALUES_DIR}/argocd-values.yaml" \
  --wait \
  --timeout 10m

info "ArgoCDインストール完了"

# ============================================================
# 起動確認
# ============================================================
section "起動確認"

kubectl get pods -n "${NAMESPACE}"
echo ""
kubectl get svc -n "${NAMESPACE}"

# ============================================================
# 初期パスワード取得
# ============================================================
section "初期管理者パスワード"

INITIAL_PASSWORD=$(kubectl get secret argocd-initial-admin-secret \
  -n "${NAMESPACE}" \
  -o jsonpath="{.data.password}" | base64 -d 2>/dev/null || echo "取得失敗 - しばらく待ってから再試行してください")

MINIKUBE_IP=$(minikube ip 2>/dev/null || echo "127.0.0.1")

info "ユーザー名: admin"
info "パスワード: ${INITIAL_PASSWORD}"
warn "初回ログイン後にパスワードを変更してください"

# ============================================================
# argocd CLI のインストール確認
# ============================================================
section "argocd CLI"

if command -v argocd >/dev/null 2>&1; then
  info "argocd CLI はインストール済みです"
else
  warn "argocd CLI が見つかりません。以下でインストールできます:"
  echo ""
  echo "  # macOS"
  echo "  brew install argocd"
  echo ""
  echo "  # Linux"
  echo "  curl -sSL -o argocd https://github.com/argoproj/argo-cd/releases/latest/download/argocd-linux-amd64"
  echo "  chmod +x argocd && sudo mv argocd /usr/local/bin/"
fi

# ============================================================
# GitOps: Application リソースの適用
# ============================================================
section "GitOps設定 (Application リソース)"

info "monitoring-stack Application を適用しますか?"
info "  対象リポジトリ: https://github.com/Oasis1994LiveForever/k8s-training.git"
info "  同期パス      : manifests/ (kustomize)"
info "  デプロイ先    : monitoring namespace"
read -rp "適用しますか? [y/N]: " apply_app

if [[ "${apply_app}" =~ ^[Yy]$ ]]; then
  kubectl apply -f "${MANIFESTS_DIR}/argocd/argocd-application-monitoring.yaml"
  info "monitoring-stack Application を適用しました"
  info "ArgoCDがGitリポジトリと同期を開始します"
else
  info "後で手動で適用する場合:"
  info "  kubectl apply -f manifests/argocd/argocd-application-monitoring.yaml"
fi

# ============================================================
# 完了メッセージ
# ============================================================
section "インストール完了"

echo ""
echo -e "${BOLD}ArgoCD UI へのアクセス方法${NC}"
echo "──────────────────────────────────────────────────────"
echo ""
echo -e "${CYAN}ブラウザでアクセス${NC}"
echo "  NodePort : http://${MINIKUBE_IP}:30808"
echo "  minikube : minikube service argocd-server -n ${NAMESPACE}"
echo "  pfwd     : kubectl port-forward svc/argocd-server -n ${NAMESPACE} 8080:80"
echo "             → http://localhost:8080"
echo ""
echo -e "${CYAN}argocd CLI でログイン${NC}"
echo "  argocd login ${MINIKUBE_IP}:30808 --insecure --username admin"
echo ""
echo -e "${CYAN}パスワード確認${NC}"
echo "  kubectl get secret argocd-initial-admin-secret \\"
echo "    -n argocd -o jsonpath=\"{.data.password}\" | base64 -d"
echo ""
echo -e "${CYAN}パスワード変更${NC}"
echo "  argocd account update-password"
echo ""
echo -e "${CYAN}Applicationの同期状態確認${NC}"
echo "  argocd app list"
echo "  argocd app sync monitoring-stack"
echo "  argocd app get monitoring-stack"
echo ""
echo -e "${CYAN}Helmリリース確認${NC}"
echo "  helm list -n ${NAMESPACE}"
echo ""
echo "──────────────────────────────────────────────────────"
echo ""
echo -e "${CYAN}GitOpsのワークフロー${NC}"
echo "  1. Windows で manifests/ を編集"
echo "  2. git commit && git push"
echo "  3. ArgoCD が自動検知してクラスタに同期"
echo "  → ArgoCDダッシュボードで差分とデプロイ状況を確認"
