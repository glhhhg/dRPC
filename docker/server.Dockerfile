FROM golang:latest

WORKDIR /app

COPY . .

RUN go build -o /server_end ./main/server.go

CMD ["/server_end", "-l", "0.0.0.0", "-p", "12000"]
