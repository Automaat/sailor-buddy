FROM golang:1.26-alpine AS builder
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /app/sailor-buddy ./cmd/api
RUN go build -o /app/seed-dev ./cmd/seed-dev

FROM alpine:3.23
RUN apk add --no-cache ca-certificates chromium
ENV CHROME_PATH=/usr/bin/chromium-browser
WORKDIR /app
COPY --from=builder /app/sailor-buddy .
COPY --from=builder /app/seed-dev .
EXPOSE 8080
CMD ["./sailor-buddy"]
