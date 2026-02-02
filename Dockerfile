FROM alpine:latest
ARG TARGETPLATFORM
RUN apk --no-cache add ca-certificates
COPY $TARGETPLATFORM/dns300 /usr/local/bin/dns300
ENTRYPOINT ["/usr/local/bin/dns300"]
