FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o bin/raggo ./cmd/server/...

FROM alpine:3.20
RUN adduser -D appuser
WORKDIR /app
COPY --from=builder /app/bin/raggo .
RUN mkdir -p uploads data && chown -R appuser:appuser /app
USER appuser
EXPOSE 8080
CMD ["./raggo"]
