# ArrayDeck — one image serving the API and the compiled web UI.

# ---- frontend ----
FROM node:22-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY web/ ./
RUN npm run build

# ---- backend ----
FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ cmd/
COPY internal/ internal/
COPY --from=web /src/web/dist internal/api/webdist
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -tags embedui \
    -ldflags "-s -w -X github.com/gazofnaz/unraid-dashboard/internal/config.Version=${VERSION}" \
    -o /out/arraydeck ./cmd/arraydeck

# ---- runtime ----
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S arraydeck && adduser -S -G arraydeck arraydeck \
    && mkdir -p /data && chown arraydeck:arraydeck /data
COPY --from=build /out/arraydeck /usr/local/bin/arraydeck

# ArrayDeck is read-only toward Docker by design. The image defaults to root
# so a directly mounted /var/run/docker.sock works on a stock Unraid host;
# for the hardened mode run with `--user` and DOCKER_HOST=tcp://docker-proxy:2375.
ENV ARRAYDECK_LISTEN=:8417 \
    DATA_DIR=/data \
    TEMPLATES_DIR=/unraid/templates

EXPOSE 8417
VOLUME ["/data"]
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s \
    CMD wget -q -O /dev/null http://127.0.0.1:8417/healthz || exit 1

ENTRYPOINT ["/usr/local/bin/arraydeck"]
