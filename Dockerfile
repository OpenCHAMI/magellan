FROM cgr.dev/chainguard/wolfi-base

RUN apk add --no-cache tini bash

# nobody 65534:65534
USER 65534:65534


COPY  magellan  /magellan
COPY  /bin/magellan.sh /magellan.sh


CMD [ "/magellan" ]

ENTRYPOINT [ "/sbin/tini", "--" ]
