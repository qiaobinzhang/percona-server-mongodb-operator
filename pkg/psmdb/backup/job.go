package backup

import (
	batchv1 "k8s.io/api/batch/v1"
	batchv1b "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/percona/percona-server-mongodb-operator/pkg/apis/psmdb/v1"
)

func BackupCronJob(backup *api.BackupTaskSpec, crName, namespace string, backupSpec api.BackupSpec, imagePullSecrets []corev1.LocalObjectReference) *batchv1b.CronJob {
	backupPod := corev1.PodSpec{
		RestartPolicy:      corev1.RestartPolicyNever,
		ImagePullSecrets:   imagePullSecrets,
		ServiceAccountName: backupSpec.ServiceAccountName,
		Containers: []corev1.Container{
			{
				Name:    "backup",
				Image:   backupSpec.Image,
				Command: []string{"sh"},
				Env: []corev1.EnvVar{
					{
						Name:  "psmdbCluster",
						Value: crName,
					},
					{
						Name: "NAMESPACE",
						ValueFrom: &corev1.EnvVarSource{
							FieldRef: &corev1.ObjectFieldSelector{
								FieldPath: "metadata.namespace",
							},
						},
					},
				},
				Args:            newBackupCronJobContainerArgs(backup),
				SecurityContext: backupSpec.ContainerSecurityContext,
			},
		},
		SecurityContext: backupSpec.PodSecurityContext,
	}

	return &batchv1b.CronJob{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "batch/v1beta1",
			Kind:       "CronJob",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      crName + "-backup-" + backup.Name,
			Namespace: namespace,
			Labels:    NewBackupCronJobLabels(crName),
		},
		Spec: batchv1b.CronJobSpec{
			Schedule:          backup.Schedule,
			ConcurrencyPolicy: batchv1b.ForbidConcurrent,
			JobTemplate: batchv1b.JobTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: NewBackupCronJobLabels(crName),
				},
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						Spec: backupPod,
					},
				},
			},
		},
	}
}

func NewBackupCronJobLabels(crName string) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       "percona-server-mongodb",
		"app.kubernetes.io/instance":   crName,
		"app.kubernetes.io/replset":    "general",
		"app.kubernetes.io/managed-by": "percona-server-mongodb-operator",
		"app.kubernetes.io/component":  "backup-schedule",
		"app.kubernetes.io/part-of":    "percona-server-mongodb",
	}
}

func newBackupCronJobContainerArgs(backup *api.BackupTaskSpec) []string {
	return []string{
		"-c",
		`curl \
			-vvv \
			-X POST \
			--cacert /run/secrets/kubernetes.io/serviceaccount/ca.crt \
			-H "Content-Type: application/json" \
			-H "Authorization: Bearer $(cat /run/secrets/kubernetes.io/serviceaccount/token)" \
			--data "{ 
				\"kind\":\"PerconaServerMongoDBBackup\",
				\"apiVersion\":\"psmdb.percona.com/v1\",
				\"metadata\":{
					\"generateName\":\"cron-${psmdbCluster:0:16}-$(date -u "+%Y%m%d%H%M%S")-\",
					\"labels\":{
						\"ancestor\":\"` + backup.Name + `\",
						\"cluster\":\"${psmdbCluster}\",
						\"type\":\"cron\"
					}
				},
				\"spec\":{
					\"psmdbCluster\":\"${psmdbCluster}\",
					\"storageName\":\"` + backup.StorageName + `\"
				}
			}" \
			https://${KUBERNETES_SERVICE_HOST}:${KUBERNETES_SERVICE_PORT}/apis/psmdb.percona.com/v1/namespaces/${NAMESPACE}/perconaservermongodbbackups`,
	}
}
