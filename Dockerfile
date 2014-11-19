FROM golang:1.3.3

MAINTAINER lonli078

add go-irc-server /go/

expose 6667

ENTRYPOINT ./go-irc-server -d

#docker run -it -p 6667:6667 --name irc-server lonli078/go-irc-server ./go-irc-server -d
#docker run -d -p 6667:6667 --name irc-server lonli078/go-irc-server ./go-irc-server -d


#RUN
#docker run -it -p 6667:6667 --rm --name my-irc-server go-irc-server go run irc.go -d

#BUILD
#docker run --rm -v "$(pwd)":/usr/src/myapp -w /usr/src/myapp golang:1.3.3 go build -v -o go-irc-server


#export GOPATH=/home/lonli/Dropbox/go && go run %f
