FROM golang:latest

WORKDIR /app

COPY . .

RUN go build -o /registry_end ./main/registry.go

CMD ["/registry_end", "-l", "0.0.0.0", "-p", "8080"]
