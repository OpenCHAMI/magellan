FROM cgr.dev/chainguard/wolfi-base

RUN apk add --no-cache tini

# nobody 65534:65534
USER 65534:65534


COPY  magellan 

CMD [ "/magellan" ]

ENTRYPOINT [ "/sbin/tini", "--" ]
