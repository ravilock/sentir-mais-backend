FROM golang:1.26.4-alpine

RUN apk add --no-cache git

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN go build -o sentir-mais-api ./cmd/sentir-mais-api/main.go

CMD ["./sentir-mais-api"]
