FROM golang:alpine AS builder

WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o d4s ./cmd/d4s/main.go

FROM scratch
COPY --from=builder /app/d4s /d4s
ENV TERM=xterm-256color
ENTRYPOINT ["/d4s"]
