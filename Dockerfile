FROM alpine:3.19.0

RUN apk add --no-cache ca-certificates && update-ca-certificates
ADD vattenfall-to-influxdb /bin/

ENTRYPOINT ["/bin/vattenfall-to-influxdb"]
