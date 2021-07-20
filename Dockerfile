# run npm install in a container with node.
FROM node:14-alpine AS js
WORKDIR /usr/src/app
COPY ./controller/app .
run apk add npm
RUN npm install

FROM golang:alpine AS builder
RUN apk update
RUN apk upgrade
RUN apk add --update gcc>=9.3.0 g++>=9.3.0 alpine-sdk linux-headers binutils-gold

WORKDIR /go/src/app/

COPY . .
COPY --from=js /usr/src/app ./controller/app
# Fetch dependencies.
RUN go get -d -v ./...
RUN go generate ./...
RUN go build -o dealbot -ldflags "-X github.com/filecoin-project/dealbot/controller.buildDate=`date -u +%d/%m/%Y@%H:%M:%S`" ./

FROM alpine
# Fetch needed k8s / aws support.
ARG AWS_IAM_AUTHENTICATOR_URL=https://amazon-eks.s3.us-west-2.amazonaws.com/1.19.6/2021-01-05/bin/linux/amd64/aws-iam-authenticator
ARG KUBECTL_URL=https://amazon-eks.s3.us-west-2.amazonaws.com/1.20.4/2021-04-12/bin/linux/amd64/kubectl
ADD ${AWS_IAM_AUTHENTICATOR_URL} /usr/local/bin/aws-iam-authenticator
ADD ${KUBECTL_URL} /usr/local/bin/kubectl
RUN apk add --update ca-certificates gettext && \
    chmod +x /usr/local/bin/kubectl && \
    chmod +x /usr/local/bin/aws-iam-authenticator

# Copy our static executable.
COPY --from=builder /go/src/app/dealbot /dealbot
ENV DEALBOT_LOG_JSON=true
ENV DEALBOT_WORKERS=10
ENV STAGE_TIMEOUT=DefaultStorage=48h,DefaultRetrieval=48h
ENV DEALBOT_MIN_FIL=-1
ENV DEALBOT_LOTUS_GATEWAY="wss://api.chain.love"
ENTRYPOINT ["/dealbot"]
