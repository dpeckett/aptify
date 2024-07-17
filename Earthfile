VERSION 0.8
FROM golang:1.22-bookworm
WORKDIR /workspace

all:
  ARG VERSION=dev
  BUILD --platform=linux/amd64 --platform=linux/arm64 +docker
  COPY (+build/aptify --GOARCH=amd64) ./dist/aptify-linux-amd64
  COPY (+build/aptify --GOARCH=arm64) ./dist/aptify-linux-arm64
  COPY (+build/aptify --GOARCH=riscv64) ./dist/aptify-linux-riscv64
  COPY (+build/aptify --GOOS=darwin --GOARCH=amd64) ./dist/aptify-darwin-amd64
  COPY (+build/aptify --GOOS=darwin --GOARCH=arm64) ./dist/aptify-darwin-arm64
  COPY (+build/aptify --GOOS=windows --GOARCH=amd64) ./dist/aptify-windows-amd64.exe
  COPY +package/*.deb ./dist/
  RUN cd dist && find . -type f | sort | xargs sha256sum >> ../sha256sums.txt
  SAVE ARTIFACT ./dist/aptify-linux-amd64 AS LOCAL dist/aptify-linux-amd64
  SAVE ARTIFACT ./dist/aptify-linux-arm64 AS LOCAL dist/aptify-linux-arm64
  SAVE ARTIFACT ./dist/aptify-linux-riscv64 AS LOCAL dist/aptify-linux-riscv64
  SAVE ARTIFACT ./dist/aptify-darwin-amd64 AS LOCAL dist/aptify-darwin-amd64
  SAVE ARTIFACT ./dist/aptify-darwin-arm64 AS LOCAL dist/aptify-darwin-arm64
  SAVE ARTIFACT ./dist/aptify-windows-amd64.exe AS LOCAL dist/aptify-windows-amd64.exe
  SAVE ARTIFACT ./sha256sums.txt AS LOCAL dist/sha256sums.txt

docker:
  FROM ghcr.io/dpeckett/debco/debian:bookworm-ultraslim
  RUN groupadd -g 65532 nonroot \
    && useradd -u 65532 -g 65532 -s /sbin/nologin -m nonroot
  COPY LICENSE /usr/share/doc/aptify/copyright
  ARG TARGETARCH
  COPY (+build/aptify --GOOS=linux --GOARCH=${TARGETARCH}) /usr/bin/aptify
  USER nonroot:nonroot
  # Make sure a mount point exists for the configuration directory (with the 
  # correct uid/gid).
  RUN mkdir -p /home/nonroot/.config/aptify
  ENTRYPOINT ["/usr/bin/aptify"]
  ARG VERSION=dev
  EXPOSE 8080/tcp 8443/tcp
  SAVE IMAGE --push ghcr.io/dpeckett/aptify:${VERSION}
  SAVE IMAGE --push ghcr.io/dpeckett/aptify:latest

build:
  ARG GOOS=linux
  ARG GOARCH=amd64
  COPY go.mod go.sum ./
  RUN go mod download
  COPY . .
  ARG VERSION=dev
  RUN CGO_ENABLED=0 go build --ldflags "-s -X 'github.com/dpeckett/aptify/internal/constants.Version=${VERSION}'" -o aptify main.go
  SAVE ARTIFACT ./aptify AS LOCAL dist/aptify-${GOOS}-${GOARCH}

tidy:
  LOCALLY
  RUN go mod tidy
  RUN go fmt ./...

lint:
  FROM golangci/golangci-lint:v1.59.1
  WORKDIR /workspace
  COPY . ./
  RUN golangci-lint run --timeout 5m ./...

test:
  COPY go.mod go.sum ./
  RUN go mod download
  COPY . .
  RUN go test -coverprofile=coverage.out -v ./...
  SAVE ARTIFACT ./coverage.out AS LOCAL coverage.out
  # A quick integration test to ensure that we are producing valid repositories.
  RUN go build .
  RUN ./aptify init-keys \
    && ./aptify build -c examples/demo.yaml -o /tmp/aptify \
    && cp /tmp/aptify/signing_key.asc /etc/apt/keyrings/aptify-keyring.asc \
    && echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/aptify-keyring.asc] file:/tmp/aptify bookworm stable" > /etc/apt/sources.list.d/demo.list \
    && apt update \
    && apt install -y hello-world \
    && /usr/bin/hello

package:
  FROM debian:bookworm
  # General build dependencies.
  RUN apt update \
    && apt install -y git curl devscripts dpkg-dev debhelper-compat
  # Go tooling.
  RUN apt install -y git dh-sequence-golang golang-any golang
  # Cross-compilation tooling.
  RUN apt install -y gcc-aarch64-linux-gnu gcc-riscv64-linux-gnu
  # Libraries dependencies.
  RUN curl -fsL -o /etc/apt/keyrings/apt-pecke-tt-keyring.asc https://apt.pecke.tt/signing_key.asc \
    && echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/apt-pecke-tt-keyring.asc] http://apt.pecke.tt $(. /etc/os-release && echo $VERSION_CODENAME) stable" > /etc/apt/sources.list.d/apt-pecke-tt.list \
    && apt update
  RUN apt install -y \
    golang-github-adrg-xdg-dev \
    golang-github-dpeckett-archivefs-dev \
    golang-github-dpeckett-compressmagic-dev \
    golang-github-dpeckett-deb822-dev \
    golang-github-dpeckett-slog-shim-dev \
    golang-github-otiai10-copy-dev \
    golang-github-protonmail-go-crypto-dev \
    golang-github-urfave-cli-v2-dev \
    golang-golang-x-crypto-dev \
    golang-golang-x-sync-dev \
    golang-golang-x-sys-dev \
    golang-gopkg-yaml.v3-dev
  RUN mkdir -p /workspace/aptify
  WORKDIR /workspace/aptify
  COPY . .
  ENV EMAIL=damian@pecke.tt
  RUN export DEBEMAIL="damian@pecke.tt" \
    && export DEBFULLNAME="Damian Peckett" \
    && export VERSION=$(git describe --tags --abbrev=0 | tr -d 'v') \
    && dch --create --package aptify --newversion "${VERSION}-1" \
      --distribution "UNRELEASED" --force-distribution  --controlmaint "Last Commit: $(git log -1 --pretty=format:'(%ai) %H %cn <%ce>')" \
    && tar -czf ../aptify_${VERSION}.orig.tar.gz .
  RUN dpkg-buildpackage -us -uc --host-arch=amd64
  RUN dpkg-buildpackage -d -us -uc --host-arch=arm64
  RUN dpkg-buildpackage -d -us -uc --host-arch=riscv64
  SAVE ARTIFACT /workspace/*.deb AS LOCAL dist/