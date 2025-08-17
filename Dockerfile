# syntax=docker/dockerfile:1

# ---------- build stage ----------
FROM golang:1.22-alpine AS build
WORKDIR /src

# 你的 Go 模块在子目录 wsbot/；go.mod 必须，go.sum 有就拷
COPY wsbot/go.* wsbot/

# 可配置代理（默认官方），按需覆盖：--build-arg GOPROXY=https://proxy.golang.org,direct
ARG GOPROXY=https://proxy.golang.org,direct
ENV GOPROXY=$GOPROXY

# 预拉依赖（会生成/更新 go.sum）
WORKDIR /src/wsbot
RUN --mount=type=cache,target=/go/pkg/mod go mod download

# 拷贝整个模块源码
WORKDIR /src
COPY wsbot ./wsbot

# 可选：显式指定要构建的 main 包（相对 wsbot/ 的包路径，如 ./cmd/wsbot）
# 不指定时会自动探测第一个 package main
ARG MAINPKG=""

# 为了兼容 IDE 解析：用 /bin/sh -c + 单行转义，避免 heredoc/复杂引号
WORKDIR /src/wsbot
RUN --mount=type=cache,target=/root/.cache/go-build /bin/sh -c '\
  set -euo pipefail; \
  pkg="${MAINPKG}"; \
  if [ -z "$pkg" ]; then \
    pkg=$(go list -f "{{if eq .Name \"main\"}}{{.ImportPath}}{{end}}" ./... | head -n 1); \
  fi; \
  test -n "$pkg" || { echo "ERROR: no main package found under wsbot/"; exit 1; }; \
  echo "Building main package: $pkg"; \
  CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o /out/wsbot "$pkg" \
'

# ---------- runtime stage ----------
FROM alpine:3.20
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /app
COPY --from=build /out/wsbot /usr/local/bin/wsbot
USER 65532:65532
ENTRYPOINT ["/usr/local/bin/wsbot"]
