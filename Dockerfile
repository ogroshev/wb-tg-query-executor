FROM golang:alpine AS builder

LABEL stage=gobuilder

ENV CGO_ENABLED 0

WORKDIR /build

ADD go.mod .
ADD go.sum .
RUN go mod download
COPY . .
RUN go build -ldflags="-s -w" -o /app/wb-tg-query-executor cmd/main.go


FROM alpine

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
RUN apk update --no-cache && apk add --no-cache tzdata
ENV TZ Europe/Moscow

WORKDIR /app
COPY --from=builder /app/wb-tg-query-executor /app/wb-tg-query-executor

CMD ["./wb-tg-query-executor"]
