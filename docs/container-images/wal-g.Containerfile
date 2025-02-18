ARG POSTGRES_VERSION=16.4.0
ARG GOLANG_VERSION=1.23.1

# build wal-g binary
FROM docker.io/library/golang:${GOLANG_VERSION}-bullseye as build-wal-g

ARG WALG_VERSION=v3.0.3

RUN apt update && apt install -y git build-essential

WORKDIR /workspace
RUN git clone https://github.com/wal-g/wal-g.git /workspace/wal-g
RUN cd /workspace/wal-g && git checkout ${WALG_VERSION} && go mod tidy && go build -o wal-g ./main/pg/...

# runner image
FROM docker.io/bitnami/postgresql:${POSTGRES_VERSION}

USER root
COPY --from=build-wal-g /workspace/wal-g/wal-g /usr/local/bin/wal-g
RUN chmod +x /usr/local/bin/wal-g

USER 1001
## Optionally, create a directory for wal-g config
# RUN mkdir -p /etc/wal-g
## add a default wal-g config file here if needed
# COPY wal-g.json /etc/wal-g/wal-g.json
## Set environment variables for wal-g
#ENV WALG_CONFIG_FILE="/etc/wal-g/wal-g.json"

# Entrypoint for the container
ENTRYPOINT [ "/opt/bitnami/scripts/postgresql/entrypoint.sh" ]
CMD [ "/opt/bitnami/scripts/postgresql/run.sh" ]
