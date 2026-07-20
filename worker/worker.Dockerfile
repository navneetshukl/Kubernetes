FROM golang:1.25.1 as builder
WORKDIR /app
# COPY go.mod go.sum ./
# RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o worker main.go

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/worker ./worker
CMD ["./worker"]