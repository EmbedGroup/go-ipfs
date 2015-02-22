FROM golang:1.4
MAINTAINER Brian Tiger Chow <btc@perfmode.com>

ADD . /go/src/github.com/jbenet/go-ipfs
RUN cd /go/src/github.com/jbenet/go-ipfs/cmd/ipfs && go install

EXPOSE 4001 5001 4002/udp

ENTRYPOINT ["ipfs"]

CMD ["daemon", "--init"]

# build:    docker build -t go-ipfs .
# run:      docker run -p 4001:4001 -p 5001:5001 go-ipfs:latest daemon --init
# run:      docker run -p 4002:4002/udp -p 4001:4001 -p 5001:5001 go-ipfs:latest daemon --init
