FROM --platform=linux/arm64 golang:1.23-bookworm AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o /build-doctor .

FROM --platform=linux/arm64 node:22-bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    git \
    make \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Install Claude Code CLI
RUN npm install -g @anthropic-ai/claude-code

COPY --from=builder /build-doctor /usr/local/bin/build-doctor

ENTRYPOINT ["build-doctor"]
