FROM golang:latest AS builder

ARG BIN=tag-monger
RUN apt-get update && \
    apt-get install  \
    binutils \
    ca-certificates \
    curl \
    git \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /go/src/tag-monger
COPY . .
RUN if [[ ! -e vendor ]]; then dep ensure && dep status; else true; fi
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o "$BIN" .
RUN strip "$BIN"

FROM ubuntu:latest
RUN apt-get update && \
    apt-get install -y ca-certificates tzdata bash && \
    rm -rf /var/lib/apt/lists/*
WORKDIR /root/
COPY --from=builder /go/src/tag-monger/$BIN /bin/$BIN
CMD ["/bin/tag-monger"]
