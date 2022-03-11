FROM golang:latest

WORKDIR /app

COPY . .
COPY server/assets/ assets

RUN go install ./...
RUN go build -o cbyge-server ./server