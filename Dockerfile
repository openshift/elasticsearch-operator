### This is a generated file from Dockerfile.in ###
#@follow_tag(openshift-golang-builder:1.14)
FROM openshift-golang-builder:v1.14.4-9 AS builder

ENV BUILD_VERSION=${CI_CONTAINER_VERSION}
ENV OS_GIT_MAJOR=${CI_X_VERSION}
ENV OS_GIT_MINOR=${CI_Y_VERSION}
ENV OS_GIT_PATCH=${CI_Z_VERSION}
ENV SOURCE_GIT_COMMIT=${CI_ELASTICSEARCH_OPERATOR_UPSTREAM_COMMIT}
ENV SOURCE_GIT_URL=${CI_ELASTICSEARCH_OPERATOR_UPSTREAM_URL}


WORKDIR /go/src/github.com/openshift/elasticsearch-operator

COPY ${REMOTE_SOURCE}/apis apis
COPY ${REMOTE_SOURCE}/controllers controllers
COPY ${REMOTE_SOURCE}/files files
COPY ${REMOTE_SOURCE}/internal internal
COPY ${REMOTE_SOURCE}/manifests manifests
COPY ${REMOTE_SOURCE}/vendor vendor
COPY ${REMOTE_SOURCE}/version version
COPY ${REMOTE_SOURCE}/.bingo ./.bingo
ADD ${REMOTE_SOURCE}/Makefile ${REMOTE_SOURCE}/main.go ${REMOTE_SOURCE}/go.mod ${REMOTE_SOURCE}/go.sum ./

RUN make build

#@follow_tag(openshift-ose-base:ubi8)
FROM openshift-ose-base:v4.0-202009120053.11408
LABEL \
        io.k8s.display-name="OpenShift elasticsearch-operator" \
        io.k8s.description="This is the component that manages an Elasticsearch cluster on a kubernetes based platform" \
        io.openshift.tags="openshift,logging,elasticsearch" \
        com.redhat.delivery.appregistry="false" \
        maintainer="AOS Logging <aos-logging@redhat.com>" \
        name="openshift/ose-elasticsearch-operator" \
        com.redhat.component="elasticsearch-operator-container" \
        io.openshift.maintainer.product="OpenShift Container Platform" \
        io.openshift.build.commit.id=${CI_ELASTICSEARCH_OPERATOR_UPSTREAM_COMMIT} \
        io.openshift.build.source-location=${CI_ELASTICSEARCH_OPERATOR_UPSTREAM_URL} \
        io.openshift.build.commit.url=${CI_ELASTICSEARCH_OPERATOR_UPSTREAM_URL}/commit/${CI_ELASTICSEARCH_OPERATOR_UPSTREAM_COMMIT} \
        version=${CI_CONTAINER_VERSION}

ENV ALERTS_FILE_PATH="/etc/elasticsearch-operator/files/prometheus_alerts.yml"
ENV RULES_FILE_PATH="/etc/elasticsearch-operator/files/prometheus_recording_rules.yml"
ENV ES_DASHBOARD_FILE="/etc/elasticsearch-operator/files/dashboards/logging-dashboard-elasticsearch.json"
ENV RUNBOOK_BASE_URL="https://docs.openshift.com/container-platform/latest/logging/troubleshooting/cluster-logging-troubleshooting-for-critical-alerts.html"

COPY --from=builder /go/src/github.com/openshift/elasticsearch-operator/bin/elasticsearch-operator /usr/bin/
COPY --from=builder /go/src/github.com/openshift/elasticsearch-operator/files/ /etc/elasticsearch-operator/files/
COPY --from=builder /go/src/github.com/openshift/elasticsearch-operator/manifests /manifests
RUN mkdir /tmp/ocp-eo && \
    chmod og+w /tmp/ocp-eo

WORKDIR /usr/bin
ENTRYPOINT ["elasticsearch-operator"]

