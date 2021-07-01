package cronjob

import (
	batchv1 "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Builder represents the type to build Cronjob objects
type Builder struct {
	cj *batchv1beta1.CronJob
}

// New returns a new Builder for Cronjob objects
func New(name, namespace string, labels map[string]string) *Builder {
	return &Builder{cj: newCronjob(name, namespace, labels)}
}

func newCronjob(name, namespace string, labels map[string]string) *batchv1beta1.CronJob {
	return &batchv1beta1.CronJob{
		TypeMeta: metav1.TypeMeta{
			Kind:       "CronJob",
			APIVersion: batchv1beta1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: batchv1beta1.CronJobSpec{
			JobTemplate: batchv1beta1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: namespace,
							Labels:    labels,
						},
					},
				},
			},
		},
	}
}

// Build returns the final Cronjob object
func (b *Builder) Build() *batchv1beta1.CronJob { return b.cj }

// WithConcurrencyPolicy sets the concurrency policy for the cronjob
func (b *Builder) WithConcurrencyPolicy(cp batchv1beta1.ConcurrencyPolicy) *Builder {
	b.cj.Spec.ConcurrencyPolicy = cp
	return b
}

// WithSuccessfulJobsHistoryLimit sets the limit for the history of successful jobs
func (b *Builder) WithSuccessfulJobsHistoryLimit(l int32) *Builder {
	b.cj.Spec.SuccessfulJobsHistoryLimit = &l
	return b
}

// WithFailedJobsHistoryLimit sets the limit for the history of failed jobs
func (b *Builder) WithFailedJobsHistoryLimit(l int32) *Builder {
	b.cj.Spec.FailedJobsHistoryLimit = &l
	return b
}

// WithSchedule sets the cronjob's schedule
func (b *Builder) WithSchedule(s string) *Builder {
	b.cj.Spec.Schedule = s
	return b
}

// WithBackoffLimit sets the cronjob's job backoff limit
func (b *Builder) WithBackoffLimit(l int32) *Builder {
	b.cj.Spec.JobTemplate.Spec.BackoffLimit = &l
	return b
}

// WithParallelism sets the cronjob's job parallelism limit
func (b *Builder) WithParallelism(p int32) *Builder {
	b.cj.Spec.JobTemplate.Spec.Parallelism = &p
	return b
}

// WithPodSpec sets the cronjob pod spec and its name
func (b *Builder) WithPodSpec(containerName string, spec *corev1.PodSpec) *Builder {
	b.cj.Spec.JobTemplate.Spec.Template.ObjectMeta.Name = containerName
	b.cj.Spec.JobTemplate.Spec.Template.Spec = *spec
	return b
}
