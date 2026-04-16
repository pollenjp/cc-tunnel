FROM mirror.gcr.io/library/golang:1.26.2-bookworm AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /cc-tmux-tunnel ./cmd/cc-tmux-tunnel

FROM mirror.gcr.io/library/debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    nodejs \
    npm \
    tmux \
    curl \
    && rm -rf /var/lib/apt/lists/*

RUN curl https://cursor.com/install -fsS > /tmp/install_cursor_cli.sh \
    && chmod +x /tmp/install_cursor_cli.sh \
    && /tmp/install_cursor_cli.sh \
    && cp ~/.local/bin/agent /usr/local/bin
RUN npm install -g @anthropic-ai/claude-code

COPY --from=builder /cc-tmux-tunnel /usr/local/bin/cc-tmux-tunnel

ENTRYPOINT ["cc-tmux-tunnel"]
