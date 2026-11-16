#!/bin/bash
cd "$(dirname "$0")"
export PATH="/opt/homebrew/bin:/usr/local/bin:/usr/local/go/bin:$HOME/go/bin:$PATH"
command -v go >/dev/null || { echo "Go not found: brew install go"; read -n1; exit 1; }
command -v npm >/dev/null || { echo "Node not found: brew install node"; read -n1; exit 1; }
(cd web && npm install --no-audit --no-fund && npm run build) && (cd backend && go run ./cmd/server)
read -n1
