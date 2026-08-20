FROM golang:1.25-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/api ./cmd/api \
    && CGO_ENABLED=0 GOOS=linux go build -o /bin/bootstrap-admin ./cmd/bootstrap-admin

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/

COPY --from=builder /bin/api .
COPY --from=builder /bin/bootstrap-admin .
EXPOSE 8080

CMD ["./api"]
