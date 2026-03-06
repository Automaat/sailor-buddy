FROM golang:1.26-alpine AS builder
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /app/sailor-buddy ./cmd/api

FROM alpine:3.23
RUN apk add --no-cache ca-certificates chromium
ENV CHROME_PATH=/usr/bin/chromium-browser
WORKDIR /app
COPY --from=builder /app/sailor-buddy .
EXPOSE 8080
CMD ["./sailor-buddy"]
