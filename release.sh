#!/bin/bash

set -e

source venv/bin/activate

echo "Release to PyPI?"
select yn in "yes" "no"; do
  case $yn in
    yes) break;;
    no) exit 2;;
  esac
done

twine upload dist/*
