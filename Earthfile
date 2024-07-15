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
  RUN cd dist && find . -type f | sort | xargs sha256sum >> ../sha256sums.txt
  SAVE ARTIFACT ./dist/aptify-linux-amd64 AS LOCAL dist/aptify-linux-amd64
  SAVE ARTIFACT ./dist/aptify-linux-arm64 AS LOCAL dist/aptify-linux-arm64
  SAVE ARTIFACT ./dist/aptify-linux-riscv64 AS LOCAL dist/aptify-linux-riscv64
  SAVE ARTIFACT ./dist/aptify-darwin-amd64 AS LOCAL dist/aptify-darwin-amd64
  SAVE ARTIFACT ./dist/aptify-darwin-arm64 AS LOCAL dist/aptify-darwin-arm64
  SAVE ARTIFACT ./dist/aptify-windows-amd64.exe AS LOCAL dist/aptify-windows-amd64.exe
  SAVE ARTIFACT ./sha256sums.txt AS LOCAL dist/sha256sums.txt

docker:
  FROM gcr.io/distroless/static-debian12:nonroot
  COPY LICENSE /usr/share/doc/aptify/copyright
  ARG TARGETARCH
  COPY (+build/aptify --GOOS=linux --GOARCH=${TARGETARCH}) /usr/bin/aptify
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
