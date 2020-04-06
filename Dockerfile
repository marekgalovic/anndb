FROM golang:1.14-alpine3.11

# Copy code
RUN mkdir -p /anndb
COPY . /anndb
WORKDIR /anndb

ENV CGO_ENABLED=0
ENV GOOS=linux
ENV GOARCH=amd64

# Build
RUN go mod download
RUN go build -o /go/bin/anndb cmd/anndb/main.go

ENTRYPOINT ["/go/bin/anndb"]