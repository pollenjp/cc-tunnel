FROM mirror.gcr.io/library/golang:1.26.2-bookworm AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /cc-remote-agent ./cmd/cc-remote-agent

FROM mirror.gcr.io/library/debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    nodejs \
    npm \
    sudo \
    && rm -rf /var/lib/apt/lists/*

# 非rootユーザー作成（--dangerously-skip-permissions は root では動作しないため）
RUN useradd -m -s /bin/bash user \
    && echo "user ALL=(ALL) NOPASSWD:ALL" >> /etc/sudoers \
    && mkdir -p /home/user/.claude \
    && chown -R user:user /home/user/.claude

RUN npm install -g @anthropic-ai/claude-code

COPY --from=builder /cc-remote-agent /usr/local/bin/cc-remote-agent

USER user

ENTRYPOINT ["cc-remote-agent"]
