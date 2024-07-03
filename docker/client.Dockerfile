FROM golang:latest

WORKDIR /app

COPY . .

RUN go build -o /client_end ./main/client.go

CMD ["/client_end", "-i", "192.168.1.10", "-p", "8080"]
