FROM golang:alpine as BUILD

WORKDIR /go/src/goapp/
COPY . .

RUN set -x \
  \
  && apk add --no-cache \
    libvirt-dev \
    g++ \
    git \
  \
  && go get -d ./... \
  && go build

FROM alpine:latest

RUN set -x \
  \
  && apk add --no-cache \
    libvirt-client \
    openssh-client

COPY --from=BUILD /go/src/goapp/goapp /

COPY entrypoint.sh /
ENTRYPOINT ["/entrypoint.sh"]
