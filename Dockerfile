# syntax=docker/dockerfile:1

FROM golang:1.23-alpine AS build

WORKDIR /src

# Dependencies first so the layer is cached until go.mod/go.sum change.
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY main.go ./
COPY cmd ./cmd
COPY internal ./internal
COPY pkg ./pkg

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/simple .

FROM alpine:3.20

RUN apk add --no-cache ca-certificates \
    && addgroup -S -g 10001 app \
    && adduser -S -D -H -u 10001 -G app app

WORKDIR /app
COPY --from=build /out/simple /usr/local/bin/simple

USER app
EXPOSE 8888

# The container listens on :8888, so probe it over loopback.
HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 \
    CMD wget -qO- http://127.0.0.1:8888/api/v1/ping >/dev/null 2>&1 || exit 1

ENTRYPOINT ["/usr/local/bin/simple"]
