FROM golang:1.25.1 as build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o main main.go

FROM alpine:latest 
WORKDIR /app
COPY --from=build /app/main .
CMD ["./main"]