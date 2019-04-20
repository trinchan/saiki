ARG GO_VERSION=1.12
FROM golang:${GO_VERSION} AS builder
WORKDIR /src
COPY . .
RUN GO111MODULE=on go build -mod=vendor

FROM debian:stretch-slim
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /src/saiki /bin/saiki
ENTRYPOINT ["/bin/saiki"]
