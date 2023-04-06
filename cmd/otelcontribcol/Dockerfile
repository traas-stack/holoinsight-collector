FROM alpine:latest as prep
RUN apk --update add ca-certificates

RUN mkdir -p /tmp

FROM scratch

ARG USER_UID=10001
USER ${USER_UID}

COPY --from=prep /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY otelcontribcol /
COPY config/config.yml /config/
COPY config/component-libraries.yml /config/
WORKDIR /

EXPOSE 4317 11800 55680 55679
ENTRYPOINT ["/otelcontribcol"]
CMD ["--config", "/config/config.yaml"]
