ARG ALPINE_VERSION=3.10
ARG GOLANG_VERSION=1.16.9

FROM golang:${GOLANG_VERSION}
WORKDIR /go/src/github.com/Decentr-net/vulcan/
COPY . .
RUN make linux

FROM alpine:${ALPINE_VERSION}
RUN apk update && apk add ca-certificates
COPY --from=0 /go/src/github.com/Decentr-net/vulcan/build/vulcan-linux-amd64 /vulcand
COPY --from=0 /go/src/github.com/Decentr-net/vulcan/build/referral-linux-amd64 /referrald
COPY static /static
COPY scripts/migrations /migrations
ENTRYPOINT [ "/vulcand" ]
