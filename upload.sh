#!/bin/bash
set -ex
cd /home/kazuhisa/work/mora-project/mora

SERVER=http://localhost:4000

# REPO_URL=https://github.com/iszk1215/mora
REPO_URL=http://localhost:3001/kazuhisa/mora

git push gitea HEAD

# -- backend --

if test -f coverage.out; then
    ./bin/mora coverage upload \
      --server ${SERVER} \
      --repo ${REPO_URL} \
      --repo-path . \
      --entry go \
      --yes \
      coverage.out
else
    echo coverge.out not found. skip
fi


# -- frontend --

LCOV="frontend/coverage/lcov.info"

if [ ! -f "$LCOV" ]; then
  echo "Error: $LCOV not found. Run 'make frontend-coverage' first."
  exit 1
fi

# lcov.info のパスは frontend/ からの相対 -> リポジトリルートからの相対に変換
sed -i 's|^SF:src/|SF:frontend/src/|g' "$LCOV"

./bin/mora coverage upload \
  --server ${SERVER} \
  --repo ${REPO_URL} \
  --repo-path . \
  --entry frontend \
  --yes \
  "$LCOV" \
  $*
