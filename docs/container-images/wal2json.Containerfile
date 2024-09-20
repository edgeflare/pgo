ARG POSTGRES_VERSION=16.4.0

# Use the specified PostgreSQL image as the base image for the builder stage
FROM docker.io/bitnami/postgresql:${POSTGRES_VERSION} AS builder

ARG WAL2JSON_BRANCH=wal2json_2_6

USER root
RUN apt-get update && apt-get install -y gcc git make && rm -rf /var/lib/apt/lists/*
RUN git clone https://github.com/eulerto/wal2json.git --branch ${WAL2JSON_BRANCH}
RUN cd wal2json && USE_PGXS=1 make && USE_PGXS=1 make install

# runner image
FROM docker.io/edgeflare/postgresql:$POSTGRES_VERSION-wal-g-v3.0.3
COPY --from=builder /opt/bitnami/postgresql/lib/wal2json.so /opt/bitnami/postgresql/lib/
USER 1001

# set wal_level=logical in postgresql.conf
ENV POSTGRESQL_WAL_LEVEL=logical
