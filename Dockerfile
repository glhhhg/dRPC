FROM golang:latest

WORKDIR /dRPC/server

COPY . .

RUN go build -o /server .

EXPOSE 12000

ENTRYPOINT ["/server", "-i", "127.0.0.1", "-p", "12000"]