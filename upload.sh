#!/bin/bash
set -e
cd /home/kazuhisa/work/mora-project/mora

LCOV="frontend/coverage/lcov.info"

if [ ! -f "$LCOV" ]; then
  echo "Error: $LCOV not found. Run 'make frontend-coverage' first."
  exit 1
fi

# lcov.info のパスは frontend/ からの相対 -> リポジトリルートからの相対に変換
sed -i 's|^SF:src/|SF:frontend/src/|g' "$LCOV"

./bin/mora coverage upload \
  --server http://localhost:4000 \
  --repo https://github.com/iszk1215/mora \
  --repo-path . \
  --entry frontend \
  --yes \
  "$LCOV"
