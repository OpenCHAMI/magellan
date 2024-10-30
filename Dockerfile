FROM chainguard/wolfi-base:latest

# Include curl in the final image for manual checks of the Redfish urls
RUN set -ex \
    && apk update \
    && apk add --no-cache curl tini \
    && rm -rf /var/cache/apk/*  \
    && rm -rf /tmp/*

# nobody 65534:65534
USER 65534:65534


COPY  magellan  /magellan


CMD [ "/magellan" ]

ENTRYPOINT [ "/sbin/tini", "--" ]
