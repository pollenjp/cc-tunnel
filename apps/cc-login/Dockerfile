FROM mirror.gcr.io/library/golang:1.26.2-bookworm AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /cc-login ./cmd/cc-login

FROM mirror.gcr.io/library/debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    nodejs \
    npm \
    sudo \
    && rm -rf /var/lib/apt/lists/*

RUN npm install -g @anthropic-ai/claude-code

# 非rootユーザー作成（claude CLIのPTYログインに必要）
RUN useradd -m -s /bin/bash user \
    && echo "user ALL=(ALL) NOPASSWD:ALL" >> /etc/sudoers \
    && mkdir -p /home/user/.claude \
    && chown -R user:user /home/user/.claude

COPY --from=builder /cc-login /usr/local/bin/cc-login

USER user
WORKDIR /home/user

EXPOSE 9092
ENTRYPOINT ["cc-login"]
