FROM golang:1.22-alpine AS builder

WORKDIR /app
COPY go.mod go.sum* ./
RUN go mod download 2>/dev/null || true
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /contextops-api .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=builder /contextops-api /usr/local/bin/contextops-api
EXPOSE 8080
ENTRYPOINT ["contextops-api"]
