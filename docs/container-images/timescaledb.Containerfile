### extension does not work


# # Stage 1: Build TimescaleDB Tools
# ARG POSTGRES_VERSION=16.4.0
# ARG GOLANG_VERSION=1.23.1

# FROM golang:${GOLANG_VERSION} AS tools
# COPY go.mod go.sum ./
# RUN apt-get update && apt-get install -y git gcc

# # Install dependencies
# RUN apt-get update && apt-get install -y git gcc musl-dev
# RUN go install github.com/timescale/timescaledb-tune/cmd/timescaledb-tune@latest && \
#     go install github.com/timescale/timescaledb-parallel-copy/cmd/timescaledb-parallel-copy@latest

# # Stage 2: Install TimescaleDB
# FROM docker.io/edgeflare/postgresql:${POSTGRES_VERSION}-wal-g-v3.0.3 AS timescaledb-builder

# # Create the missing directory
# USER root
# RUN mkdir -p /var/lib/apt/lists/partial

# # Install dependencies
# RUN apt-get update && apt-get install -y build-essential git gcc musl-dev ca-certificates postgresql-server-dev-all postgresql-plpython3

# COPY --from=tools /go/bin/* /usr/local/bin/

# RUN set -ex \
#     && apt-get update \
#     && apt-get install -y postgresql-common \
#     && sh /usr/share/postgresql-common/pgdg/apt.postgresql.org.sh -y \
#     && apt-get install -y postgresql-16-timescaledb \
#     && apt-get clean \
#     && rm -rf /var/lib/apt/lists/*

# # Stage 3: Final Image with TimescaleDB
# FROM docker.io/edgeflare/postgresql:${POSTGRES_VERSION}-wal-g-v3.0.3

# # Copy the installed TimescaleDB files and configurations
# COPY --from=timescaledb-builder /usr/lib/postgresql/16/lib/timescaledb-*.so /usr/local/lib/postgresql/
# COPY --from=timescaledb-builder /usr/share/postgresql/16/extension/timescaledb--*.sql /usr/local/share/postgresql/extension/
# COPY --from=timescaledb-builder /usr/share/postgresql/16/extension/timescaledb.control /usr/local/share/postgresql/extension/

# # Expose the PostgreSQL port
# EXPOSE 5432

# USER 1001

# # Entrypoint and command
# ENTRYPOINT ["/opt/bitnami/scripts/postgresql/entrypoint.sh"]
# CMD ["/opt/bitnami/scripts/postgresql/run.sh"]
