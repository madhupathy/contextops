FROM golang:1.22-alpine AS builder

WORKDIR /app
COPY go.mod go.sum* ./
RUN go mod download 2>/dev/null || true
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /contextops .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates docker-cli docker-cli-compose
COPY --from=builder /contextops /usr/local/bin/contextops
ENTRYPOINT ["contextops"]
