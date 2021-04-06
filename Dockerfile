FROM registry.ci.openshift.org/ocp/builder:rhel-8-golang-1.15-openshift-4.7 AS builder
WORKDIR /go/src/github.com/openshift/elasticsearch-operator
COPY apis apis
COPY controllers controllers
COPY files files
COPY internal internal
COPY manifests manifests
COPY vendor vendor
COPY version version
ADD Makefile main.go ./
RUN go build -o bin/elasticsearch-operator main.go

FROM registry.ci.openshift.org/ocp/4.7:base

ENV ALERTS_FILE_PATH="/etc/elasticsearch-operator/files/prometheus_alerts.yml"
ENV RULES_FILE_PATH="/etc/elasticsearch-operator/files/prometheus_recording_rules.yml"
ENV ES_DASHBOARD_FILE="/etc/elasticsearch-operator/files/dashboards/logging-dashboard-elasticsearch.json"

COPY --from=builder /go/src/github.com/openshift/elasticsearch-operator/bin/elasticsearch-operator /usr/bin/
COPY --from=builder /go/src/github.com/openshift/elasticsearch-operator/files/ /etc/elasticsearch-operator/files/
COPY --from=builder /go/src/github.com/openshift/elasticsearch-operator/manifests /manifests
RUN mkdir /tmp/ocp-eo && \
    chmod og+w /tmp/ocp-eo

WORKDIR /usr/bin
ENTRYPOINT ["elasticsearch-operator"]

LABEL io.k8s.display-name="OpenShift elasticsearch-operator" \
      io.k8s.description="This is the component that manages an Elasticsearch cluster on a kubernetes based platform" \
      io.openshift.tags="openshift,logging,elasticsearch" \
      com.redhat.delivery.appregistry=true \
      maintainer="AOS Logging <aos-logging@redhat.com>"
