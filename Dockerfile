FROM golang:1.14-alpine3.11

# Copy code
RUN mkdir -p /anndb
COPY . /anndb
WORKDIR /anndb

# Build
RUN CGO_ENABLED=0 go mod download
RUN CGO_ENABLED=0 go build -o /go/bin/anndb cmd/anndb/main.go

ENTRYPOINT ["/go/bin/anndb"]