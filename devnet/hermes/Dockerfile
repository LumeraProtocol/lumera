FROM golang:1.24.7-bookworm AS builder

ARG HERMES_VERSION=v1.13.3
ARG HERMES_TARGET=x86_64-unknown-linux-gnu
ARG HERMES_DIST=hermes-${HERMES_VERSION}-${HERMES_TARGET}.tar.gz
ARG HERMES_DIR=hermes-${HERMES_VERSION}-${HERMES_TARGET}
ARG IBCGO_VERSION=v10.3.0

RUN apt-get -o Acquire::Check-Date=false -o Acquire::Check-Valid-Until=false update \
    && apt-get install -y --no-install-recommends ca-certificates curl git build-essential \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /tmp

RUN mkdir -p /out \
    && curl -sSfL "https://github.com/informalsystems/hermes/releases/download/${HERMES_VERSION}/${HERMES_DIST}" -o "${HERMES_DIST}" \
    && tar -xzf "${HERMES_DIST}" \
    && install -m 0755 "$(find . -maxdepth 2 -type f -name hermes -print -quit)" /out/hermes \
    && rm -rf "${HERMES_DIST}" "${HERMES_DIR}"

RUN git clone https://github.com/cosmos/ibc-go.git \
    && cd ibc-go \
    && git checkout "${IBCGO_VERSION}" \
    && make build \
    && install -m 0755 ./build/simd /out/simd \
    && rm -rf /tmp/ibc-go

FROM debian:trixie-slim

ARG HERMES_VERSION

ENV HERMES_CONFIG="/root/.hermes/config.toml" \
    SIMD_HOME="/root/.simd"

RUN apt-get -o Acquire::Check-Date=false -o Acquire::Check-Valid-Until=false update \
    && apt-get install -y --no-install-recommends \
        bash \
        ca-certificates \
        curl \
        procps \
        crudini \
        net-tools \
        iputils-ping \
        lnav \
        mc \
        jq \
        libssl3 \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /out/hermes /usr/local/bin/hermes
COPY --from=builder /out/simd /usr/local/bin/simd

WORKDIR /root

COPY scripts/ /root/scripts/
COPY config.toml /root/scripts/hermes-config-template.toml

RUN chmod 0755 /root/scripts/*.sh

EXPOSE 26656 26657 1317 9090

ENTRYPOINT ["/bin/bash", "/root/scripts/hermes-start.sh"]
