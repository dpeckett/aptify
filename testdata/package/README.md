# Example Package

This is an example debian package to demonstrate the usage of `aptify`.

## Prerequisites

```shell
sudo apt update
sudo apt install -y gcc-aarch64-linux-gnu libc6-dev-arm64-cross
```

## Building Source Package

To build the source package, run the following command:

```bash
debuild -S -us -uc
```

## Building Binary Packages

To build a binary package, run the following command:

```bash
dpkg-buildpackage -b -us -uc
dpkg-buildpackage -b -us -uc --host-arch=arm64
```