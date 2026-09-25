FROM --platform=$BUILDPLATFORM node:22-alpine AS web
WORKDIR /build
RUN apk add --no-cache python3 make g++
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

FROM --platform=$BUILDPLATFORM golang:1.25.13-alpine AS server
ARG TARGETOS
ARG TARGETARCH
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ cmd/
COPY internal/ internal/
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath -ldflags='-s -w' -o /out/visto ./cmd/server

FROM alpine:3.21
LABEL org.opencontainers.image.source="https://github.com/afonsocraposo/visto"
RUN apk add --no-cache tzdata && addgroup -S visto && adduser -S visto -G visto
COPY --from=server /out/visto /usr/local/bin/visto
COPY --from=web /build/dist /app/web
RUN mkdir /data && chown visto:visto /data
USER visto
ENV VISTO_DATABASE_PATH=/data/visto.db VISTO_WEB_DIR=/app/web VISTO_LISTEN_ADDR=:8080
VOLUME ["/data"]
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/visto"]
