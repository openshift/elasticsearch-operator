FROM quay.io/operator-framework/upstream-opm-builder AS opm-builder

FROM registry.ci.openshift.org/ocp/4.7:base

LABEL operators.operatorframework.io.index.database.v1=/database/index.db

RUN yum install -q -y sqlite && \
    yum clean -q all

COPY --from=opm-builder /bin/opm /usr/bin/opm
COPY --from=opm-builder /bin/grpc_health_probe /usr/bin/grpc_health_probe

ADD /olm_deploy/scripts/catalog-init.sh /usr/bin/catalog-init.sh
ADD /olm_deploy/scripts/catalog-run.sh /usr/bin/catalog-run.sh
ADD /database/index.db /registry/index.db
RUN chmod -R 775 /registry

EXPOSE 50051
WORKDIR /registry
ENTRYPOINT ["/usr/bin/catalog-run.sh"]
CMD ["/registry/index.db"]
