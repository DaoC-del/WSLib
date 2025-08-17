# syntax=docker/dockerfile:1

# ---- build stage ----
FROM golang:1.22-alpine AS build
WORKDIR /src

# 你的 Go 模块在子目录 wsbot/
# 同时兼容存在/不存在 go.sum 的情况
COPY wsbot/go.mod wsbot/
COPY wsbot/go.sum wsbot/ 2>/dev/null || true

# 可选：国内代理更稳（不需要可删）
ENV GOPROXY=https://goproxy.cn,direct

# 预拉依赖（会在镜像层里生成/更新 go.sum）
WORKDIR /src/wsbot
RUN --mount=type=cache,target=/go/pkg/mod go mod download

# 拷贝整个模块源码
WORKDIR /src
COPY wsbot ./wsbot

# 在模块内自动寻找 main 包并构建（适配 cmd/* 或其他子目录结构）
WORKDIR /src/wsbot
RUN --mount=type=cache,target=/root/.cache/go-build \
    set -eux; \
    mainpkg="$(go list -f '{{if eq .Name "main"}}{{.ImportPath}}{{end}}' ./... | head -n 1)"; \
    test -n "$mainpkg" || (echo "ERROR: no main package found under wsbot/" && exit 1); \
    echo "Building main package: $mainpkg"; \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
      go build -trimpath -ldflags='-s -w' -o /out/wsbot "$mainpkg"

# ---- runtime stage ----
FROM alpine:3.20
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /app
COPY --from=build /out/wsbot /usr/local/bin/wsbot
USER 65532:65532
ENTRYPOINT ["/usr/local/bin/wsbot"]
