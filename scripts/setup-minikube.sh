#!/usr/bin/env bash
# =============================================================================
# minikube セットアップスクリプト
# 用途: 学習環境としてのminikubeクラスタを起動する
# 実行: bash scripts/setup-minikube.sh
# =============================================================================

set -euo pipefail

# カラー出力用
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

info()    { echo -e "${GREEN}[INFO]${NC} $*"; }
warn()    { echo -e "${YELLOW}[WARN]${NC} $*"; }
error()   { echo -e "${RED}[ERROR]${NC} $*"; exit 1; }

# -------------------------------------------
# 前提条件チェック
# -------------------------------------------
info "前提条件を確認しています..."

command -v minikube >/dev/null 2>&1 || error "minikubeがインストールされていません"
command -v kubectl  >/dev/null 2>&1 || error "kubectlがインストールされていません"
command -v docker   >/dev/null 2>&1 || error "Dockerがインストールされていません"

# Docker Desktopが起動しているか確認
docker info >/dev/null 2>&1 || error "Docker Desktopが起動していません。起動してから再実行してください"

info "前提条件OK"

# -------------------------------------------
# minikubeクラスタの起動
# -------------------------------------------
info "minikubeクラスタを起動します..."
info "  ドライバー: docker"
info "  CPU: 4コア"
info "  メモリ: 8GB"
info "  Kubernetesバージョン: v1.28.0"

minikube start \
  --driver=docker \
  --cpus=4 \
  --memory=8192 \
  --kubernetes-version=v1.28.0 \
  --addons=metrics-server

info "minikube起動完了"

# -------------------------------------------
# アドオンの有効化
# -------------------------------------------
info "アドオンを有効化しています..."

# metrics-serverはstart時に有効化済み
# ダッシュボードは必要時に手動で起動 (常時起動は不要)

info "アドオン設定完了"

# -------------------------------------------
# 動作確認
# -------------------------------------------
info "クラスタの状態を確認しています..."

kubectl get nodes
echo ""
kubectl get pods -A

# -------------------------------------------
# 完了メッセージ
# -------------------------------------------
echo ""
info "=============================="
info "minikubeセットアップ完了!"
info "=============================="
echo ""
echo "次のステップ:"
echo "  bash scripts/install-observability.sh"
echo ""
echo "便利なコマンド:"
echo "  minikube dashboard          # KubernetesダッシュボードをブラウザUIで起動"
echo "  minikube stop               # クラスタ停止"
echo "  minikube delete             # クラスタ削除"
echo "  kubectl get all -n monitoring # 全リソース確認"
