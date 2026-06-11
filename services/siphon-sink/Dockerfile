FROM golang:alpine AS builder
WORKDIR /app
COPY . .
RUN go work sync
RUN CGO_ENABLED=0 GOOS=linux go build -o siphon-sink ./services/siphon-sink/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/siphon-sink .
CMD ["./siphon-sink"]
