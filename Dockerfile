FROM golang:1.22-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git ca-certificates tzdata

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o servidor ./cmd/api

FROM alpine:latest

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata

RUN addgroup -g 1000 app && adduser -D -u 1000 -G app app
USER app

COPY --from=builder /app/servidor .

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget -qO- http://localhost:8080/health || exit 1

CMD ["./servidor"]
