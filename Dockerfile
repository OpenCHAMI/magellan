#
# STAGE 1: Build
#

FROM golang:1.27.0 AS builder
ARG CGO_ENABLED=0
WORKDIR /magellan

COPY go.mod go.sum ./
RUN go mod download
COPY . .

RUN make clean
RUN make

#
# STAGE 2: Application
#

FROM --platform=$TARGETPLATFORM chainguard/wolfi-base:latest

# Include curl in the final image for manual checks of the Redfish urls
RUN set -ex \
    && apk update \
    && apk add --no-cache curl tini \
    && rm -rf /var/cache/apk/*  \
    && rm -rf /tmp/*

# nobody 65534:65534
USER 65534:65534


COPY --from=builder /magellan/magellan /magellan


CMD [ "/magellan" ]

ENTRYPOINT [ "/sbin/tini", "--" ]
