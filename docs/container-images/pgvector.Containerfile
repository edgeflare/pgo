ARG POSTGRES_VERSION=16.4.0

FROM docker.io/bitnami/postgresql:$POSTGRES_VERSION AS builder

ARG PGVECTOR_BRANCH=v0.7.4

USER root
RUN apt update && apt install -y build-essential git

WORKDIR /tmp/pgvector
RUN git clone --branch $PGVECTOR_BRANCH https://github.com/pgvector/pgvector.git /tmp/pgvector
RUN make && make install

# runner image
FROM docker.io/edgeflare/postgresql:$POSTGRES_VERSION-wal-g-v3.0.3

COPY --from=builder /tmp/pgvector/vector.so /opt/bitnami/postgresql/lib/
COPY --from=builder /tmp/pgvector/vector.control /opt/bitnami/postgresql/share/extension/
COPY --from=builder /tmp/pgvector/sql/*.sql /opt/bitnami/postgresql/share/extension/

## alternative using pgvector image
# FROM docker.io/pgvector/pgvector:0.7.4-pg16 AS builder
# FROM docker.io/bitnami/postgresql:$POSTGRES_VERSION

# COPY --from=builder /usr/lib/postgresql/16/lib/vector.so /opt/bitnami/postgresql/lib/
# COPY --from=builder /usr/share/postgresql/16/extension/vector* /opt/bitnami/postgresql/share/extension/
