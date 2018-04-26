FROM golang:1.9.5-alpine3.7 as builder

ARG BIN=tag-monger
RUN apk --update --no-cache add \
    binutils \
    ca-certificates \
    curl \
    git \
    && rm -rf /root/.cache
RUN curl -sSL https://raw.githubusercontent.com/golang/dep/master/install.sh \
    | sh
WORKDIR /go/src/github.com/lsst-sqre/tag-monger
COPY . .
RUN if [[ ! -e vendor ]]; then dep ensure && dep status; else true; fi
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o "$BIN" .
RUN strip "$BIN"

FROM alpine:3.7
RUN apk --update --no-cache add ca-certificates tzdata bash \
    && rm -rf /root/.cache
WORKDIR /root/
COPY --from=builder /go/src/github.com/lsst-sqre/tag-monger/$BIN /bin/$BIN
CMD ["/bin/tag-monger"]
