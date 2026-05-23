FROM mirror.gcr.io/library/golang:1.26.2-bookworm AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /cc-remote-agent ./cmd/cc-remote-agent \
    && CGO_ENABLED=0 go build -o /cc-hook-bridge ./cmd/cc-hook-bridge

FROM mirror.gcr.io/library/debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    nodejs \
    npm \
    sudo \
    && rm -rf /var/lib/apt/lists/*

RUN npm install -g @anthropic-ai/claude-code

# 非rootユーザー作成（--dangerously-skip-permissions は root では動作しないため）
RUN useradd -m -s /bin/bash user \
    && echo "user ALL=(ALL) NOPASSWD:ALL" >> /etc/sudoers \
    && mkdir -p /home/user/.claude \
    && chown -R user:user /home/user/.claude

COPY --from=builder /cc-remote-agent /usr/local/bin/cc-remote-agent
COPY --from=builder /cc-hook-bridge /usr/local/bin/cc-hook-bridge
COPY cmd/cc-hook-bridge/settings.json /home/user/.claude/settings.json
RUN chown user:user /home/user/.claude/settings.json

USER user
WORKDIR /home/user
ENTRYPOINT ["cc-remote-agent"]
