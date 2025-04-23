#/bin/sh

rm -rf dist

source venv/bin/activate

GOOS=darwin GOARCH=arm64 python3 -m build
GOOS=darwin GOARCH=amd64 python3 -m build
GOOS=linux GOARCH=amd64 GOOS_EXTRA="many" python3 -m build
GOOS=linux GOARCH=arm64 GOOS_EXTRA="many" python3 -m build
GOOS=linux GOARCH=amd64 GOOS_EXTRA="musl" python3 -m build
GOOS=linux GOARCH=arm64 GOOS_EXTRA="musl" python3 -m build

# twine upload dist/*
