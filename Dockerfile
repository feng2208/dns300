FROM alpine:3.23.3
ARG TARGETPLATFORM

RUN apk --no-cache add ca-certificates libcap && \
    mkdir /data && \
    chown nobody: /data
COPY --chown=nobody:nogroup $TARGETPLATFORM/dns300 /usr/local/bin/dns300
RUN setcap 'cap_net_bind_service=+eip' /usr/local/bin/dns300

WORKDIR /data
EXPOSE 53/tcp 53/udp

ENTRYPOINT ["/usr/local/bin/dns300"]
