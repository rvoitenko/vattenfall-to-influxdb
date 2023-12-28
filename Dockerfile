FROM alpine:3.16.2

RUN apk add --no-cache ca-certificates && update-ca-certificates
ADD vattenfall-to-influxdb /bin/

ENTRYPOINT ["/bin/vattenfall-to-influxdb"]
