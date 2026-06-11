FROM golang:alpine AS builder
WORKDIR /app
COPY . .
RUN go work sync
RUN CGO_ENABLED=0 GOOS=linux go build -o siphon-stream-api ./services/siphon-stream-api/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/siphon-stream-api .
EXPOSE 8080
CMD ["./siphon-stream-api"]
