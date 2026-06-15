#!/bin/bash
set -e
cd "$(dirname "$0")"

# Find go binary
GO=$(which go 2>/dev/null || echo /opt/homebrew/bin/go)

echo "=== 构建前端 ==="
cd frontend
npm install --silent
npm run build
cd ..

echo "=== 构建后端 ==="
cd backend
$GO build -o clipboooard .
cd ..

echo ""
echo "=== 启动服务 ==="
cd backend
exec ./clipboooard
