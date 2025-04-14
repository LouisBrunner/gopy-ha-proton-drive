# gopy-ha-proton-drive

Go-based Python package for [ha-proton-drive](https://github.com/LouisBrunner/ha-proton-drive) used to interact with Proton Drive.

## Build

This repository contains a Go library which can be built into a Python package using [`gopy`](https://github.com/go-python/gopy).

It can be installed directly through `pip` thanks to [`setuptools-gopy`](https://github.com/LouisBrunner/setuptools-gopy):

```
proton @ git+https://github.com/LouisBrunner/gopy-ha-proton-drive@main
```

## Disclaimers

* It is built specifically for [ha-proton-drive](https://github.com/LouisBrunner/ha-proton-drive), thus it is unlikely to be useful for your use-case.
* Due to API changes, it relies on 2 Go forks:

  - https://github.com/LouisBrunner/go-proton-api: `main` branch
  - https://github.com/LouisBrunner/Proton-API-Bridge: `main` branch
