FROM golang:alpine AS builder
WORKDIR /app
COPY . .
RUN go work sync
RUN CGO_ENABLED=0 GOOS=linux go build -o siphon-ingress ./services/siphon-ingress/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/siphon-ingress .
EXPOSE 50051
CMD ["./siphon-ingress"]
