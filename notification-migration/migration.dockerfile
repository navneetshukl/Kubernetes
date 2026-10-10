# Build stage
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY main.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -o migration main.go

FROM alpine:3.20
WORKDIR /app
COPY --from=builder /app/migration .
ENTRYPOINT ["./migration"]