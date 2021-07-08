# run npm install in a container with node.
FROM node:14-alpine AS js
WORKDIR /usr/src/app
COPY ./controller/app .
run apk add npm
RUN npm install

FROM golang:alpine AS builder
RUN apk update
RUN apk upgrade
RUN apk add --update gcc>=9.3.0 g++>=9.3.0 alpine-sdk linux-headers

WORKDIR /go/src/app/

COPY . .
COPY --from=js /usr/src/app ./controller/app
# Fetch dependencies.
RUN go get -d -v ./...
RUN go generate ./...
RUN go build -o dealbot -ldflags "-X github.com/filecoin-project/dealbot/controller.buildDate=`date -u +%d/%m/%Y@%H:%M:%S`" ./

FROM alpine
# Copy our static executable.
COPY --from=builder /go/src/app/dealbot /dealbot
ENV DEALBOT_LOG_JSON=true
ENV DEALBOT_WORKERS=10
ENV STAGE_TIMEOUT=DefaultStorage=48h,DefaultRetrieval=48h
ENV DEALBOT_MIN_FIL=-1
ENV DEALBOT_DAEMON_DRIVER=kubernetes
ENTRYPOINT ["/dealbot"]
