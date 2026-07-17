# --- Stage 1: Build Stage ---
FROM golang:1.24-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o main ./cmd/api

# --- Stage 2: Final Slim Run Stage ---
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy the compiled binary
COPY --from=builder /app/main .

# Copy the migration folder so the binary can read the SQL files!
COPY --from=builder /app/db/migration ./db/migration

EXPOSE 8080

CMD ["./main"]