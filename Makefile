WITH_VIRTUALENV ?= yes
PIP_CMD = venv/bin/pip
PY_CMD = venv/bin/python

ifneq ($(WITH_VIRTUALENV),yes)
PIP_CMD := pip3
PY_CMD := python3
endif

all: vet lint format
.PHONY: all

venv:
	if [ ! -d venv ]; then \
	  python3 -m venv venv; \
	fi

install: venv
	$(PIP_CMD) install -r requirements-dev.txt
.PHONY: install

lint: install
	$(PY_CMD) -m ruff check .
.PHONY: lint

format: install
	$(PY_CMD) -m ruff format . --check
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
	GOOS=linux GOARCH=arm64 $(PY_CMD) -m build
	mkdir -p ../ha-proton-drive/gopy-wheels
	cp dist/gopy_ha_proton_drive-*-py3-none-manylinux2014_aarch64.whl ../ha-proton-drive/gopy-wheels/
.PHONY: dev

release: install clean
	GOOS=darwin GOARCH=arm64 $(PY_CMD) -m build
	GOOS=darwin GOARCH=amd64 $(PY_CMD) -m build
	GOOS=linux GOARCH=amd64 $(PY_CMD) -m build
	GOOS=linux GOARCH=arm64 $(PY_CMD) -m build
	GOOS=linux GOARCH=amd64 GOOS_LINUX="musllinux_1_1" $(PY_CMD) -m build
	GOOS=linux GOARCH=arm64 GOOS_LINUX="musllinux_1_1" $(PY_CMD) -m build
	bash release.sh
.PHONY: release

clean:
	rm -rf dist .ruff_cache
.PHONY: clean

clean-all: clean
	rm -rf venv
.PHONY: clean-all
