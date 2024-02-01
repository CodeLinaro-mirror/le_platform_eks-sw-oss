FROM public.ecr.aws/docker/library/ubuntu:22.04

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
       software-properties-common \
       rsyslog bash rsync \
    && apt-get clean

COPY --chown=bin csm-x100-fconfig.sh /usr/bin
COPY --chmod=777 DuMgrConfig.txt /etc/.
COPY --chmod=777 splane_cfg.conf /etc/.
COPY --chmod=777 vf-count.conf /etc/.
CMD ["/usr/bin/csm-x100-fconfig.sh"]
