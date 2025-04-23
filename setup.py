import os
import subprocess

from setuptools import setup
from setuptools.command.bdist_wheel import bdist_wheel
from setuptools.command.build import build


class MyBDistWheel(bdist_wheel):
    def finalize_options(self):
        if "GOOS" in os.environ and "GOARCH" in os.environ:
            plat = os.environ["GOOS"]
            arch = os.environ["GOARCH"]
            plat_extra = os.getenv("GOOS_EXTRA", "")
            if arch == "amd64":
                arch = "x86_64"
            if arch == "arm64":
                arch = "aarch64"
            if arch == "386":
                arch = "i386"
            if plat == "darwin":
                plat = "macos_11_0"
            self.plat_name = f"{plat_extra}{plat}_{arch}"

        super().finalize_options()


class MyBuild(build):
    def run(self):
        output = os.path.join(self.build_lib, "proton", "_go_exec")
        subprocess.run(
            [
                "go",
                "build",
                "-o",
                f"{output}",
                "github.com/LouisBrunner/gopy-ha-proton-drive/go/cmd/go_exec",
            ],
            check=True,
        )
        build.run(self)


setup(cmdclass={"build": MyBuild, "bdist_wheel": MyBDistWheel})
