# start a golang base image, version latest
FROM golang:latest

#switch to our app directory
RUN mkdir -p /go/src/app  
WORKDIR /go/src/app

#copy the source files
COPY main.go /go/src/app

#disable crosscompiling 
ENV CGO_ENABLED=0

#compile linux only
ENV GOOS=linux

#build the binary with debug information removed
RUN go get app
RUN go build  -ldflags '-w -s' -a -installsuffix cgo -o bubble 
