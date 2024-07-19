FROM golang:latest

WORKDIR /app

COPY . .

RUN go build -o /server .

EXPOSE 8080

ENTRYPOINT ["/server"]