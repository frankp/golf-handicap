# syntax=docker/dockerfile:1.7

FROM node:22.23.1-alpine3.24 AS web-builder
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.26.5-alpine3.24 AS go-builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . ./
COPY --from=web-builder /src/web/dist ./web/dist
ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} \
    go build -buildvcs=false -trimpath -ldflags="-s -w" -o /out/golf-server ./cmd/server

FROM alpine:3.24.1
RUN apk add --no-cache ca-certificates tzdata \
    && mkdir -p /app/web /data \
    && chown -R 99:100 /data
WORKDIR /app
COPY --from=go-builder /out/golf-server /app/golf-server
COPY --from=web-builder /src/web/dist /app/web/dist

ENV GOLF_ADDR=:8080 \
    GOLF_DB=/data/golf.db \
    GOLF_STATIC=/app/web/dist \
    TZ=Australia/Melbourne

USER 99:100
EXPOSE 8080
VOLUME ["/data"]
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -q -O /dev/null http://127.0.0.1:8080/api/health || exit 1
ENTRYPOINT ["/app/golf-server"]
