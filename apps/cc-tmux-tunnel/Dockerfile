FROM mirror.gcr.io/library/golang:1.24-bookworm AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /cc-tmux-tunnel ./cmd/cc-tmux-tunnel

FROM mirror.gcr.io/library/debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    tmux \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /cc-tmux-tunnel /usr/local/bin/cc-tmux-tunnel

EXPOSE 9090
ENTRYPOINT ["cc-tmux-tunnel"]
CMD ["-addr", ":9090"]
