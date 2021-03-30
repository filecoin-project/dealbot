FROM golang:alpine AS builder
RUN apk update
RUN apk upgrade
RUN apk add --update gcc>=9.3.0 g++>=9.3.0 alpine-sdk

WORKDIR /go/src/app/

COPY . .
# Fetch dependencies.
RUN go get -d -v ./...
RUN go build -o dealbot ./

FROM alpine
# Copy our static executable.
COPY --from=builder /go/src/app/dealbot /dealbot
ENTRYPOINT ["/dealbot"]