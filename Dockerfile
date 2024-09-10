FROM golang:1.23.1-bullseye

WORKDIR /usr/src/
RUN apt-get update && apt-get upgrade -y && apt-get install -y upx-ucl && apt-get clean
RUN go install golang.org/x/vuln/cmd/govulncheck@latest && go install github.com/goreleaser/nfpm/v2/cmd/nfpm@latest && go clean -modcache
ENV DOCKER=true
ENV GOCACHE=/usr/src/.go/cache
ENV GOPATH=/usr/src/.go/path
