FROM golang:alpine AS build
WORKDIR /go/src/app
COPY . .
RUN export GO111MODULE=on && export GOPROXY=https://goproxy.cn && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -v -o cachelayer -a -ldflags "-s -w" -tags timetzdata main.go && \
    cp cachelayer /


FROM alpine
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.tuna.tsinghua.edu.cn/g' /etc/apk/repositories && apk add --update --no-cache ca-certificates
COPY --from=build /cachelayer /
WORKDIR /data
ENTRYPOINT ["/cachelayer"]
EXPOSE 6060
