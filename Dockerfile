FROM golang:1.22 AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o opslink .

FROM alpine:3.24

RUN apk add --no-cache tzdata ca-certificates
COPY --from=builder /app/opslink /app/opslink

ENTRYPOINT ["/app/opslink"]