# syntax=docker/dockerfile:1

# ---- build stage ----
FROM golang:1.22-alpine AS build
WORKDIR /src

# 你的 Go 模块在子目录 wsbot/
# 先只拷贝 go.mod（没有 go.sum 也不报错）
COPY wsbot/go.mod ./wsbot/go.mod

# 可选：国内源更稳（不需要可删除这一行）
ENV GOPROXY=https://goproxy.cn,direct

# 进入模块目录预拉依赖（会在镜像层内生成 go.sum）
WORKDIR /src/wsbot
RUN --mount=type=cache,target=/go/pkg/mod go mod download

# 再拷贝整个模块源码
WORKDIR /src
COPY wsbot ./wsbot

# 在模块目录内构建（这里用 .，因为已在 wsbot/ 里）
WORKDIR /src/wsbot
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o /out/wsbot .

# ---- runtime stage ----
FROM alpine:3.20
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /app
COPY --from=build /out/wsbot /usr/local/bin/wsbot
USER 65532:65532
ENTRYPOINT ["/usr/local/bin/wsbot"]
