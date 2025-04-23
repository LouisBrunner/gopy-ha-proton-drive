#/bin/sh

rm -rf dist

source venv/bin/activate

GOOS=darwin GOARCH=arm64 python3 -m build
GOOS=darwin GOARCH=amd64 python3 -m build
GOOS=linux GOARCH=amd64 python3 -m build
GOOS=linux GOARCH=arm64 python3 -m build
GOOS=linux GOARCH=amd64 GOOS_LINUX="musllinux_1_1" python3 -m build
GOOS=linux GOARCH=arm64 GOOS_LINUX="musllinux_1_1" python3 -m build

echo "Release to PyPI?"
select yn in "yes" "no"; do
  case $yn in
    yes) break;;
    no) exit;;
  esac
done

twine upload dist/*
