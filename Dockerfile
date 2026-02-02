FROM alpine:latest
RUN apk --no-cache add ca-certificates
COPY dns300 /usr/local/bin/dns300
ENTRYPOINT ["/usr/local/bin/dns300"]
