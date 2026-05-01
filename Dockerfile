FROM golang:1.24.2-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /bin/subtitle-delivery .

FROM alpine:3.21

WORKDIR /app

COPY --from=builder /bin/subtitle-delivery /app/subtitle-delivery
COPY .env.dev ./.env.dev
COPY .env.prod ./.env.prod

EXPOSE 8080

CMD ["/app/subtitle-delivery"]