FROM alpine:3.16.2
EXPOSE 9000

RUN apk add --no-cache ca-certificates && update-ca-certificates
ADD vattenfall-to-influxdb /bin/

ENTRYPOINT ["/bin/vattenfall-to-influxdb"]