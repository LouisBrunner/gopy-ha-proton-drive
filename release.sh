#/bin/sh

rm -rf dist

source venv/bin/activate

docker build - -t ghpd/builder <<EOF
FROM --platform=linux/amd64 python:3.13.3-alpine3.21
RUN apk add --no-cache build-base
EOF

SETUPTOOLS_GOPY_PLAT_NAME=musllinux_1_1_x86_64 SETUPTOOLS_GOPY_XCOMPILE_IMAGE=ghpd/builder python3 -m build -n
SETUPTOOLS_GOPY_PLAT_NAME=linux-x86_64 python3 -m build -n
for f in dist/*-linux_x86_64*
do
    mv "$f" "${f/linux_x86_64/manylinux2014_x86_64}"
done
SETUPTOOLS_GOPY_PLAT_NAME=linux-aarch64 python3 -m build -n
for f in dist/*-linux_aarch64*
do
    mv "$f" "${f/linux_aarch64/manylinux2014_aarch64}"
done

# twine upload dist/*
