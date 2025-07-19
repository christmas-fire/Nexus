FROM golang:1.23-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /app/server ./cmd/server/main.go

FROM alpine:3.21.3

WORKDIR /app

RUN addgroup -S appgroup && adduser -S appuser -G appgroup

COPY --from=builder /app/server .

COPY --from=builder /app/web ./web

RUN chown -R appuser:appgroup /app

USER appuser

EXPOSE 8080
EXPOSE 8081

CMD ["./server"]