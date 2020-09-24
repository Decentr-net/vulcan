ARG ALPINE_VERSION=3.10
ARG GOLANG_VERSION=1.14.2

FROM golang:1.14.2
WORKDIR /go/src/github.com/Decentr-net/vulcan/
COPY . .
RUN make linux

FROM alpine:$ALPINE_VERSION
RUN apk update && apk add ca-certificates
COPY --from=0 /go/src/github.com/Decentr-net/vulcan/build/vulcan-linux-amd64 /vulcand
ADD scripts/migrations/postgres /scripts/migrations/postgres
COPY static /static
ENTRYPOINT [ "/vulcand" ]
