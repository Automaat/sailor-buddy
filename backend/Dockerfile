FROM golang:1.26-alpine AS builder
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /app/sailor-buddy ./cmd/api

FROM golang:1.26-alpine
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /app/sailor-buddy .
COPY --from=builder /app/internal/db/migrations ./internal/db/migrations
EXPOSE 8080
CMD ["./sailor-buddy"]
