FROM mirror.gcr.io/library/golang:1.26.2-bookworm AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /cc-tunnel ./cmd/cc-tunnel

FROM gcr.io/distroless/static-debian12

COPY --from=builder /cc-tunnel /usr/local/bin/cc-tunnel

ENTRYPOINT ["cc-tunnel"]
