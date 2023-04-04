FROM golang:1.20

WORKDIR /app

COPY go.mod .
COPY go.sum .
RUN go mod download

COPY api api
COPY cmd cmd
COPY internal internal
COPY pkg pkg

RUN go build -o chat-server ./cmd/chat/main.go

EXPOSE 8980

ENTRYPOINT ["./chat-server"]