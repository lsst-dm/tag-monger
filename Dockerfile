FROM golang:1.9.5-alpine3.7 as builder

RUN apk --update --no-cache add ca-certificates curl git \
    && rm -rf /root/.cache
RUN curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh
WORKDIR /go/src/github.com/lsst-sqre/tag-monger
COPY . .
RUN dep ensure
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o app .

FROM alpine:3.7
RUN apk --update --no-cache add ca-certificates tzdata bash \
    && rm -rf /root/.cache
WORKDIR /root/
COPY --from=builder /go/src/github.com/lsst-sqre/tag-monger/app .
CMD ["./app"]
