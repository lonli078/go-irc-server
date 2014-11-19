FROM golang:1.3.3

MAINTAINER lonli078

add . /go/

ENTRYPOINT go build -v -o go-irc-server

