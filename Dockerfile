FROM golang:1.19.3-bullseye

RUN mkdir /app 
ADD . /app/
WORKDIR /app 

ENV PATH="${PATH}:/app"

RUN go mod download && go mod verify
RUN go build -o phisherman_server main.go

CMD ["./phisherman_server"]