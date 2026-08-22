FROM golang:1.25.1 as builder
WORKDIR /app

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o sidecar main.go

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/sidecar ./sidecar
CMD ["./sidecar"]