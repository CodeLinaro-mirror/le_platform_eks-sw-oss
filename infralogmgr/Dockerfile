FROM public.ecr.aws/docker/library/ubuntu:22.04 as builder

ARG DEBIAN_FRONTEND=noninteractive
RUN apt-get update \
    && apt-get install -y --no-install-recommends \
       ca-certificates \
       git build-essential cmake \
       libboost-all-dev

COPY infralogmgr /tmp/infralogmgr

WORKDIR /tmp
RUN git clone -q  https://github.com/daniele77/cli.git \
    && cd cli \
    && git reset --hard 1b94e8a89f31e1d685f738801fe28ca115ccabd2 \
    && mkdir build \
    && cd build \
    && cmake .. \
    && make install \
    && cd ../..

RUN git clone -q https://github.com/p-ranav/tabulate.git \
    && cd tabulate \
    && git reset --hard b35db4cce50a4b296290b0ae827305cdeb23751e \
    && mkdir build \
    && cd build \
    && cmake .. \
    && make install \
    && cd ../..

RUN cd infralogmgr \
    && mkdir build \
    && cd build \
    && cmake .. \
    && make \
    && cd ../..

FROM public.ecr.aws/docker/library/ubuntu:22.04
COPY --from=builder /tmp/infralogmgr/build/infralogmgr /opt/data/csm/tools/infralogmgr/infralogmgr
COPY ./CPM_X100_DMC.cfg /opt/data/csm/tools/infralogmgr/CPM_X100_DMC.cfg

USER 65532:65532
