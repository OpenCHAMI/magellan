FROM alpine:latest as base

COPY src/requirements.txt /app/requirements.txt

USER root

RUN apk -U upgrade
RUN apk add --no-cache tini bash
RUN apk add --no-cache --update \
        build-base \
        python3 \
        python3-dev \
        py3-pip \
        openssl \
        openssl-dev \
        libffi-dev \
        gcc \
        musl-dev \
        cargo \
        curl \
        --force-missing-repositories

# added python venv for MacOS builds with brew
RUN python3 -m venv ./env \
    && chmod 777 -R ./env \
    && ./env/bin/activate
RUN ./env/bin/python3 -m pip install --upgrade \
        pip \
        setuptools \
    && ./env/bin/python3 -m pip install wheel \
    && ./env/bin/python3 -m pip install -r /app/requirements.txt
RUN apk del \
        build-base \
        gcc \
        python3-dev \
        openssl-dev \
        libffi-dev \
        musl-dev \
        cargo

# Insert our emulator extentions
COPY src /app
COPY mockups /app/api_emulator/redfish/static

EXPOSE 5000
ENV MOCKUPFOLDER="public-rackmount1"
ENV AUTH_CONFIG=""
ENV PORT=5000
ENV XNAME="x3000c0s0b0"
ENV MAC_SCHEMA=""
WORKDIR /app
ENTRYPOINT ["/env/bin/python3"]
CMD ["emulator.py"]