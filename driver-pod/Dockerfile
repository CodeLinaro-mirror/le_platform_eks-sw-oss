FROM registry.access.redhat.com/ubi8/ubi-init

ARG KERNEL_VERSION=5.15.120
RUN yum install -y kmod && yum clean all
RUN mkdir -p /lib/modules/${KERNEL_VERSION}

COPY host-drivers/mhi.ko /lib/modules/${KERNEL_VERSION}/
COPY host-drivers/mhi_pci.ko /lib/modules/${KERNEL_VERSION}/
COPY host-drivers/mhi_uci.ko /lib/modules/${KERNEL_VERSION}/
COPY host-drivers/wwan.ko /lib/modules/${KERNEL_VERSION}/
COPY host-drivers/wwan_mhi.ko /lib/modules/${KERNEL_VERSION}/
COPY host-drivers/mhi_net.ko /lib/modules/${KERNEL_VERSION}/
COPY host-drivers/csm_dp.ko /lib/modules/${KERNEL_VERSION}/

WORKDIR /lib/modules/${KERNEL_VERSION}
RUN echo "softdep csm_dp pre: mhi mhi_uci mhi_net wwan wwan_mhi mhi_pci" > /etc/modprobe.d/softdep.conf
COPY csm-x100kmod-load.service /etc/systemd/system/
COPY --chown=bin csm-x100kmod-load.sh /usr/bin
COPY --chown=bin csm-x100kmod-unload.sh /usr/bin

RUN systemctl enable csm-x100kmod-load;
VOLUME [ "/sys/fs/cgroup" ]
CMD ["/usr/sbin/init"]
