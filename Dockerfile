FROM golang:1.26-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/mygo ./cmd/server

FROM alpine:3.22

WORKDIR /app

RUN apk add --no-cache ca-certificates wget

COPY --from=builder /out/mygo /app/mygo
COPY migrations /app/migrations

RUN mkdir -p /app/uploads

EXPOSE 8080

CMD ["/app/mygo"]
