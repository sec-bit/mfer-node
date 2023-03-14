FROM golang:1.19-alpine as builder

#install build deps
RUN apk add --no-cache gcc musl-dev linux-headers git

WORKDIR /mfer-node

COPY go.mod go.sum ./
# download go deps
RUN go mod download

ADD . .
# build the binary
RUN go build ./cmd/mfer-node
# fresh start
FROM alpine:latest
RUN apk add --no-cache ca-certificates
# copy the binary from the builder
COPY --from=builder /mfer-node/mfer-node /usr/local/bin/mfer-node

EXPOSE 10545

CMD ["mfer-node"]