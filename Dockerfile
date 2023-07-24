FROM golang:1.20-alpine AS builder
WORKDIR /app
COPY . .

RUN go build -o ./bin/ ./...

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/bin /app/bin
ENTRYPOINT [ "/app/bin/proteus" ]