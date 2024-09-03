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
  COPY (+package/*.deb --GOARCH=amd64) ./dist/
  COPY (+package/*.deb --GOARCH=arm64) ./dist/
  COPY (+package/*.deb --GOARCH=riscv64) ./dist/
  RUN cd dist && find . -type f | sort | xargs sha256sum >> ../sha256sums.txt
  SAVE ARTIFACT ./dist/* AS LOCAL dist/
  SAVE ARTIFACT ./sha256sums.txt AS LOCAL dist/sha256sums.txt

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
  ENV GOTOOLCHAIN=go1.22.1
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
    && ./aptify build -c examples/demo.yaml -d /tmp/aptify \
    && echo "deb [arch=$(dpkg --print-architecture) signed-by=/tmp/aptify/signing_key.asc] file:/tmp/aptify bookworm stable" > /etc/apt/sources.list.d/demo.list \
    && apt update \
    && apt install -y hello-world \
    && /usr/bin/hello

docker:
  FROM ghcr.io/dpeckett/debco/debian:bookworm-ultraslim
  ENV container=docker
  COPY LICENSE /usr/share/doc/aptify/copyright
  ARG TARGETARCH
  COPY (+build/aptify --GOOS=linux --GOARCH=${TARGETARCH}) /usr/bin/aptify
  # Make sure a mount point exists for the configuration directory (with the 
  # correct uid/gid).
  RUN mkdir -p /home/nonroot/.config/aptify
  ENTRYPOINT ["/usr/bin/aptify"]
  ARG VERSION=dev
  EXPOSE 8080/tcp 8443/tcp
  SAVE IMAGE --push ghcr.io/dpeckett/aptify:${VERSION}
  SAVE IMAGE --push ghcr.io/dpeckett/aptify:latest

package:
  FROM debian:bookworm
  # Use bookworm-backports for newer golang versions
  RUN echo "deb http://deb.debian.org/debian bookworm-backports main" > /etc/apt/sources.list.d/backports.list
  RUN apt update
  # Tooling
  RUN apt install -y git curl devscripts dpkg-dev debhelper-compat git-buildpackage libfaketime dh-sequence-golang \
    golang-any=2:1.22~3~bpo12+1 golang-go=2:1.22~3~bpo12+1 golang-src=2:1.22~3~bpo12+1 \
    gcc-aarch64-linux-gnu gcc-riscv64-linux-gnu
  RUN curl -fsL -o /etc/apt/keyrings/apt-pecke-tt-keyring.asc https://apt.pecke.tt/signing_key.asc \
    && echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/apt-pecke-tt-keyring.asc] http://apt.pecke.tt $(. /etc/os-release && echo $VERSION_CODENAME) stable" > /etc/apt/sources.list.d/apt-pecke-tt.list \
    && apt update
  # Build Dependencies
  RUN apt install -y \
    golang-github-adrg-xdg-dev \
    golang-github-dpeckett-archivefs-dev \
    golang-github-dpeckett-deb822-dev \
    golang-github-dpeckett-telemetry-dev \
    golang-github-dpeckett-uncompr-dev \
    golang-github-otiai10-copy-dev \
    golang-github-pierrec-lz4-dev=4.1.18-1~bpo12+1 \
    golang-github-protonmail-go-crypto-dev \
    golang-github-urfave-cli-v2-dev \
    golang-golang-x-crypto-dev \
    golang-golang-x-sync-dev \
    golang-golang-x-sys-dev \
    golang-gopkg-yaml.v3-dev
  RUN mkdir -p /workspace/aptify
  WORKDIR /workspace/aptify
  COPY . .
  RUN if [ -n "$(git status --porcelain)" ]; then echo "Please commit your changes."; exit 1; fi
  RUN if [ -z "$(git describe --tags --exact-match 2>/dev/null)" ]; then echo "Current commit is not tagged."; exit 1; fi
  COPY debian/scripts/generate-changelog.sh /usr/local/bin/generate-changelog.sh
  RUN chmod +x /usr/local/bin/generate-changelog.sh
  ENV DEBEMAIL="damian@pecke.tt"
  ENV DEBFULLNAME="Damian Peckett"
  RUN /usr/local/bin/generate-changelog.sh
  RUN VERSION=$(git describe --tags --abbrev=0 | tr -d 'v') \
    && tar -czf ../aptify_${VERSION}.orig.tar.gz --exclude-vcs .
  ARG GOARCH
  RUN dpkg-buildpackage -d -us -uc --host-arch=${GOARCH}
  SAVE ARTIFACT /workspace/*.deb AS LOCAL dist/
