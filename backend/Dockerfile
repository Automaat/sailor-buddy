FROM golang:1.27-alpine AS builder
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /app/sailor-buddy ./cmd/api
RUN go build -o /app/seed-dev ./cmd/seed-dev

FROM golang:1.27-alpine AS dev
RUN apk add --no-cache ca-certificates chromium git
ENV CHROME_PATH=/usr/bin/chromium-browser
RUN go install github.com/air-verse/air@v1.67.4
WORKDIR /app
EXPOSE 8080
CMD ["air", "-c", ".air.toml"]

FROM alpine:3.24
# Runs as an unprivileged user so Chromium keeps its sandbox on for PDF
# renders. The fixed numeric UID lets operators chown mounted upload volumes
# and lets Kubernetes runAsNonRoot verify the user.
RUN apk add --no-cache ca-certificates chromium \
    && addgroup -S -g 10001 sailor \
    && adduser -S -D -u 10001 -G sailor -h /home/sailor sailor
ENV CHROME_PATH=/usr/bin/chromium-browser
WORKDIR /app
COPY --from=builder /app/sailor-buddy .
COPY --from=builder /app/seed-dev .
# Default SAILOR_UPLOAD_DIR is ./uploads; create it owned by the app user so
# a fresh named volume mounted here inherits that ownership.
RUN install -d -o sailor -g sailor /app/uploads
USER 10001:10001
EXPOSE 8080
CMD ["./sailor-buddy"]
