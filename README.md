# gopy-ha-proton-drive

Go-based Python package for [ha-proton-drive](https://github.com/LouisBrunner/ha-proton-drive) used to interact with Proton Drive.

The Proton API is accessed through a Go executable embedded within the Python package. Due to limitations of Go shared libraries on some platforms and architectures (notably Linux amd64), this is the only way to get it working.

## Installation

It can be installed directly through `pip`:

```
gopy-ha-proton-drive==0.0.3
# or
gopy-ha-proton-drive @ git+https://github.com/LouisBrunner/gopy-ha-proton-drive@main
```

## Disclaimers

* It used to be built through [`gopy`](https://github.com/go-python/gopy) but due to compilation issues this was dropped.
* It is built specifically for [ha-proton-drive](https://github.com/LouisBrunner/ha-proton-drive), thus it is unlikely to be useful for your use-case.
* Due to API changes, it relies on 2 Go forks:

  - https://github.com/LouisBrunner/go-proton-api: `main` branch
  - https://github.com/LouisBrunner/Proton-API-Bridge: `main` branch
