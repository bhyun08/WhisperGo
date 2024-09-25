FROM golang:1.23.1-alpine AS build

ENV GO111MODULE=on

WORKDIR /app

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY . .

RUN go build -o server .

FROM alpine:latest

WORKDIR /root/
COPY --from=build /app/server .

EXPOSE 8082

CMD ["./server"]