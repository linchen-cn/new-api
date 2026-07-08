FROM registry.cn-hangzhou.aliyuncs.com/free2walk/bun:1 AS builder

WORKDIR /build/web
ENV BUN_CONFIG_REGISTRY=https://registry.npmmirror.com
COPY web/package.json web/bun.lock ./
COPY web/default/package.json ./default/package.json
COPY web/classic/package.json ./classic/package.json
# 使用 npm 替代 bun
RUN npm config set registry https://registry.npmmirror.com && \
    npm install --fetch-retries=5 --fetch-retry-mintimeout=20000 --fetch-retry-maxtimeout=120000
COPY ./web/default ./default
COPY ./VERSION /build/VERSION
RUN cd default && DISABLE_ESLINT_PLUGIN='true' VITE_REACT_APP_VERSION=$(cat /build/VERSION) bun run build

FROM registry.cn-hangzhou.aliyuncs.com/free2walk/bun:1 AS builder-classic

WORKDIR /build/web
ENV BUN_CONFIG_REGISTRY=https://registry.npmmirror.com
COPY web/package.json web/bun.lock ./
COPY web/default/package.json ./default/package.json
COPY web/classic/package.json ./classic/package.json
# 使用 npm 替代 bun
RUN npm config set registry https://registry.npmmirror.com && \
    npm install --fetch-retries=5 --fetch-retry-mintimeout=20000 --fetch-retry-maxtimeout=120000
COPY ./web/classic ./classic
COPY ./VERSION /build/VERSION
RUN cd classic && VITE_REACT_APP_VERSION=$(cat /build/VERSION) bun run build

FROM registry.cn-hangzhou.aliyuncs.com/free2walk/golang:1.26.1-alpine AS builder2
ENV GO111MODULE=on CGO_ENABLED=0

ARG TARGETOS
ARG TARGETARCH
ENV GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64}
ENV GOEXPERIMENT=greenteagc

WORKDIR /build

ADD go.mod go.sum ./
RUN go mod download

COPY . .
COPY --from=builder /build/web/default/dist ./web/default/dist
COPY --from=builder-classic /build/web/classic/dist ./web/classic/dist
RUN go build -ldflags "-s -w -X 'github.com/QuantumNous/new-api/common.Version=$(cat VERSION)'" -o new-api

FROM registry.cn-hangzhou.aliyuncs.com/free2walk/debian:bookworm-slim

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates tzdata libasan8 wget \
    && rm -rf /var/lib/apt/lists/* \
    && update-ca-certificates

COPY --from=builder2 /build/new-api /
COPY LICENSE NOTICE THIRD-PARTY-LICENSES.md /licenses/
EXPOSE 3000
WORKDIR /data
ENTRYPOINT ["/new-api"]
