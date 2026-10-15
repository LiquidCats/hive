ARG GO_VERSION=1.27.1

FROM golang:${GO_VERSION}-alpine

RUN apk update && apk add --no-cache build-base ca-certificates && update-ca-certificates
RUN go install github.com/air-verse/air@latest

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

CMD ["air", "-c", ".air.conf"]
