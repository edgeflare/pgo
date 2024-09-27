ARG POSTGRES_VERSION=16.4.0

FROM docker.io/edgeflare/postgresql:$POSTGRES_VERSION-wal-g-v3.0.3

ARG AGE_VERSION=1.5.0

USER root
RUN apt update && apt install -y git build-essential libreadline-dev zlib1g-dev flex bison

RUN git clone --branch release/PG16/${AGE_VERSION} https://github.com/apache/age.git /tmp/age && \
    cd /tmp/age && \
    make install

USER 1001

# TODO: make slimmer image with two stages, or copy build artifacts from apache/age to edgeflare/postgresql