# syntax=docker/dockerfile:1

FROM golang:1.26.0-alpine AS builder

WORKDIR /src
RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/api ./cmd/api

FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app

COPY --from=builder /out/api /usr/local/bin/api

EXPOSE 8080

CMD ["api"]
