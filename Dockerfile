# ---- Frontend build ----
FROM node:22-alpine AS web-build
WORKDIR /web
COPY web/package.json web/package-lock.json* ./
RUN npm install
COPY web/ ./
RUN npm run build

# ---- Go build ----
FROM golang:1.23-bookworm AS go-build
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
COPY --from=web-build /web/dist ./web/dist
RUN CGO_ENABLED=0 GOOS=linux go build -buildvcs=false -ldflags="-s -w" -o /out/wyldetrace ./cmd/wyldetrace

# ---- Runtime ----
FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates iputils-ping \
    && rm -rf /var/lib/apt/lists/* \
    && useradd -r -u 10001 wyldetrace
WORKDIR /app
COPY --from=go-build /out/wyldetrace /app/wyldetrace
COPY db/migrations /app/db/migrations
USER wyldetrace
EXPOSE 8080
ENTRYPOINT ["/app/wyldetrace"]
