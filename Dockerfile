FROM golang:latest AS builder

WORKDIR /dRPC/registry

COPY . .

RUN go build -o /registry .

EXPOSE 12000

ENTRYPOINT ["/registry", "-i", "0.0.0.0", "-p", "12000"]