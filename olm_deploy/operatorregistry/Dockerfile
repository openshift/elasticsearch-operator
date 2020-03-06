FROM quay.io/operator-framework/upstream-registry-builder AS registry-builder

FROM registry.svc.ci.openshift.org/openshift/release:golang-1.12

WORKDIR /
COPY manifests/ /manifests

# TODO: update this when we remove the 0*.yaml files from the repo
RUN rm /manifests/0*.yaml /manifests/art.yaml /manifests/*/image-references

COPY olm_deploy/scripts/registry-init.sh /scripts/

COPY --from=registry-builder /build/bin/initializer /usr/bin/initializer
COPY --from=registry-builder /build/bin/registry-server /usr/bin/registry-server
COPY --from=registry-builder /bin/grpc_health_probe /usr/bin/grpc_health_probe
