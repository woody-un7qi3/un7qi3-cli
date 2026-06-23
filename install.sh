#!/usr/bin/env bash
# uq 단일 바이너리 설치 스크립트.
# Go/make 없이 GitHub Releases 의 raw 바이너리를 받아 ~/.local/bin/uq 로 설치한다.
#
#   gh api repos/un7qi3inc/un7qi3-cli/contents/install.sh \
#     -H "Accept: application/vnd.github.raw" | bash
#
# 이후 업데이트는 `uq update` 로 한다.
set -euo pipefail

REPO="un7qi3inc/un7qi3-cli"
BIN_DIR="${UQ_BIN_DIR:-$HOME/.local/bin}"

# 1) gh 설치 확인
if ! command -v gh >/dev/null 2>&1; then
  cat >&2 <<'EOF'
[필요] GitHub CLI(gh)가 설치돼 있지 않습니다.

  brew install gh        # 설치
  gh auth login          # 로그인 (un7qi3inc 접근 권한 필요)

설치·로그인 후 이 스크립트를 다시 실행하세요.
EOF
  exit 1
fi

# 2) gh 인증 확인
if ! gh auth status >/dev/null 2>&1; then
  cat >&2 <<'EOF'
[필요] gh 가 로그인돼 있지 않습니다. 먼저 인증하세요.

  gh auth login          # un7qi3inc 조직 레포 접근 권한 필요

로그인 후 이 스크립트를 다시 실행하세요.
EOF
  exit 1
fi

# 3) OS / 아키텍처 판별 → 릴리스 자산 이름(uq_<os>_<arch>)
os=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$(uname -m)" in
  arm64 | aarch64) arch=arm64 ;;
  x86_64)          arch=amd64 ;;
  *) echo "지원하지 않는 아키텍처: $(uname -m)" >&2; exit 1 ;;
esac
asset="uq_${os}_${arch}"

# 4) 최신 릴리스 자산 다운로드 → 설치
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
echo "최신 릴리스에서 ${asset} 다운로드 중..."
gh release download --repo "$REPO" --pattern "$asset" --dir "$tmp"
mkdir -p "$BIN_DIR"
install -m 0755 "$tmp/$asset" "$BIN_DIR/uq"

echo "설치 완료: $BIN_DIR/uq"
"$BIN_DIR/uq" version || true

# 5) PATH 안내
case ":$PATH:" in
  *":$BIN_DIR:"*) ;;
  *) echo "[안내] $BIN_DIR 가 PATH 에 없습니다. 셸 설정에 추가하세요:"; \
     echo "  export PATH=\"$BIN_DIR:\$PATH\"" ;;
esac
