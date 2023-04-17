FROM alpine:latest

#  install curl for docker compose health check
RUN apk --update add ca-certificates curl bash
RUN mkdir -p /tmp

ARG USER_UID=10001
USER ${USER_UID}

COPY otelcontribcol /
COPY config/config.yml /config/
COPY config/component-libraries.yml /config/
WORKDIR /

EXPOSE 4317 11800 55680 55679
ENTRYPOINT ["/otelcontribcol"]
CMD ["--config", "/config/config.yml"]
