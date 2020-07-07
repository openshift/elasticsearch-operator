package utils

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func NewSecret(secretName string, namespace string, data map[string][]byte) *corev1.Secret {
	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: corev1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
		},
		Type: "Opaque",
		Data: data,
	}
}

func getMockedSecret(clusterName, namespace string) *corev1.Secret {
	return NewSecret(
		clusterName,
		namespace,
		map[string][]byte{
			"elasticsearch.key": GetFileContents("test/files/elasticsearch.key"),
			"elasticsearch.crt": GetFileContents("test/files/elasticsearch.crt"),
			"logging-es.key":    GetFileContents("test/files/logging-es.key"),
			"logging-es.crt":    GetFileContents("test/files/logging-es.crt"),
			"admin-key":         GetFileContents("test/files/system.admin.key"),
			"admin-cert":        GetFileContents("test/files/system.admin.crt"),
			"admin-ca":          GetFileContents("test/files/ca.crt"),
		},
	)
}
