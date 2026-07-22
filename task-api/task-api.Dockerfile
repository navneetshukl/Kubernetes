FROM golang:1.25.1 as builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod tidy
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o task-api .

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/task-api ./task-api
EXPOSE 8080
CMD ["./task-api"]