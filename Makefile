all: vet lint format
.PHONY: all

venv:
	virtualenv venv

install: venv
	venv/bin/pip install -r requirements-dev.txt
.PHONY: install

lint: install
	venv/bin/python -m ruff check .
.PHONY: lint

format: install
	venv/bin/python -m ruff format . --check
.PHONY: format

download:
	go mod download
.PHONY: download

vet: download
	gofmt -d -e -s .
	go vet ./...
	go tool staticcheck ./...
.PHONY: vet

dev:
	GOOS=linux GOARCH=arm64 venv/bin/python -m build
	mkdir -p ../ha-proton-drive/gopy-wheels
	cp dist/gopy_ha_proton_drive-*-py3-none-manylinux2014_aarch64.whl ../ha-proton-drive/gopy-wheels/
.PHONY: dev

release: install
	bash release.sh
.PHONY: release
