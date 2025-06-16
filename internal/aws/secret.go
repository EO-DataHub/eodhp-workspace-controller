package aws

import (
	"context"
	"fmt"

	"github.com/EO-DataHub/eodhp-workspace-controller/api/v1alpha1"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type SecretReconciler struct {
	client.Client
	AWS AWSClient
}

func (r *SecretReconciler) Reconcile(ctx context.Context,
	spec *v1alpha1.WorkspaceSpec,
	status *v1alpha1.WorkspaceStatus) error {

	// Nothing to reconcile
	return nil
}

// Teardown deletes the AWS Secrets Manager secret associated with the workspace.
// It uses the secret name format "<namespace>-<prefix>" to identify the secret.
// If the secret does not exist, it logs a message and returns nil.
func (r *SecretReconciler) Teardown(ctx context.Context,
	spec *v1alpha1.WorkspaceSpec,
	status *v1alpha1.WorkspaceStatus) error {

	log := log.FromContext(ctx)

	svc := secretsmanager.New(r.AWS.sess)

	secretName := fmt.Sprintf("%s-%s", spec.Namespace, r.AWS.config.Prefix)

	_, err := svc.DeleteSecret(&secretsmanager.DeleteSecretInput{
		SecretId:                   aws.String(secretName),
		ForceDeleteWithoutRecovery: aws.Bool(true),
	})
	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok && awsErr.Code() == secretsmanager.ErrCodeResourceNotFoundException {
			log.Info("Secret already deleted or never existed", "SecretName", secretName)
			return nil
		}
		return fmt.Errorf("failed to delete secret %s: %w", secretName, err)
	}

	log.Info("Secret deleted", "SecretName", secretName)
	return nil
}
