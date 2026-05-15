FROM golang:1.21-alpine AS builder
WORKDIR /build
COPY . .
RUN go build -ldflags="-s -w" -o Any2Claude ./cmd/any2claude

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /build/Any2Claude .
COPY cmd/any2claude/embed/config.json .

EXPOSE 8089 8090

ENTRYPOINT ["./Any2Claude", "-host", "0.0.0.0"]
