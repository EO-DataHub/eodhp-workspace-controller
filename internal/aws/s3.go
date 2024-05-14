/*
Copyright 2024 Telespazio UK.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package aws

import (
	"context"
	"html/template"
	"os"
	"strings"

	corev1alpha1 "github.com/UKEODHP/workspace-controller/api/v1alpha1"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/s3control"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type S3Reconciler struct {
	client.Client
	AWS AWSClient
}

func (r *S3Reconciler) Reconcile(ctx context.Context,
	spec *corev1alpha1.WorkspaceSpec,
	status *corev1alpha1.WorkspaceStatus) error {

	log := log.FromContext(ctx)

	bucketStatuses := make([]corev1alpha1.S3BucketStatus, 0,
		len(spec.AWS.S3.Buckets))

	for _, bucket := range spec.AWS.S3.Buckets {
		bucketStatus := &corev1alpha1.S3BucketStatus{
			Name: bucket.Name,
		}
		bucketStatuses = append(bucketStatuses, *bucketStatus)

		if err := r.ReconcileS3AccessPoint(ctx, &bucket,
			bucketStatus); err != nil {
			log.Error(err, "Failed creating S3 Access Point", "bucket", bucket)
		}

		if err := r.ReconcileS3RolePolicy(ctx, &bucket,
			bucketStatus, spec.AWS.RoleName); err != nil {
			log.Error(err, "Failed creating S3 Role Policy", "bucket", bucket)
		}

	}
	status.AWS.S3.Buckets = bucketStatuses
	return nil
}

func (r *S3Reconciler) Teardown(ctx context.Context,
	spec *corev1alpha1.WorkspaceSpec,
	status *corev1alpha1.WorkspaceStatus) error {

	log := log.FromContext(ctx)

	bucketStatuses := make([]corev1alpha1.S3BucketStatus, 0,
		len(spec.AWS.S3.Buckets))

	for _, bucket := range spec.AWS.S3.Buckets {
		bucketStatus := &corev1alpha1.S3BucketStatus{
			Name: bucket.Name,
		}
		bucketStatuses = append(bucketStatuses, *bucketStatus)

		if status.AWS.RoleName != "" {
			svc := iam.New(r.AWS.sess)
			if _, err := svc.DeleteRolePolicy(&iam.DeleteRolePolicyInput{
				PolicyName: &bucket.AccessPointName,
				RoleName:   &status.AWS.RoleName,
			}); err == nil {
				log.Info("Deleted EFS Role Policy", "policy",
					&bucket.AccessPointName, "role", &status.AWS.RoleName)
			} else {
				log.Error(err, "Failed to delete EFS Role Policy",
					"policy", &bucket.AccessPointName, "role", &status.AWS.RoleName)
			}
		}

		if err := r.DeleteS3AccessPoint(ctx, &bucket,
			bucketStatus); err != nil {
			log.Error(err, "Failed to delete S3 Access Point", "bucket", bucket)
		}
	}
	status.AWS.S3.Buckets = bucketStatuses
	return nil
}

func (r *S3Reconciler) ReconcileS3AccessPoint(ctx context.Context,
	bucket *corev1alpha1.S3Bucket,
	status *corev1alpha1.S3BucketStatus) error {

	log := log.FromContext(ctx)
	svc := s3control.New(r.AWS.sess)

	if ap, err := svc.GetAccessPoint(&s3control.GetAccessPointInput{
		AccountId: aws.String(r.AWS.config.AccountID),
		Name:      aws.String(bucket.AccessPointName),
	}); err == nil {
		// Access point exists.
		status.AccessPointARN = *ap.AccessPointArn
	} else if aerr, ok := err.(awserr.Error); ok {
		if aerr.Code() == "NoSuchAccessPoint" {
			// Access point does not exist. Create it.
			if ap, err := svc.CreateAccessPoint(&s3control.CreateAccessPointInput{
				AccountId: aws.String(r.AWS.config.AccountID),
				Bucket:    aws.String(bucket.Name),
				Name:      aws.String(bucket.AccessPointName),
			}); err == nil {
				log.Info("Created S3 Access point", "bucket", bucket)
				status.AccessPointARN = *ap.AccessPointArn
			} else {
				return err
			}
		} else {
			return err
		}
	} else {
		return err
	}
	return nil
}

func (r *S3Reconciler) DeleteS3AccessPoint(ctx context.Context,
	bucket *corev1alpha1.S3Bucket,
	status *corev1alpha1.S3BucketStatus) error {

	log := log.FromContext(ctx)
	svc := s3control.New(r.AWS.sess)

	if _, err := svc.DeleteAccessPoint(&s3control.DeleteAccessPointInput{
		AccountId: aws.String(r.AWS.config.AccountID),
		Name:      aws.String(bucket.AccessPointName),
	}); err == nil {
		log.Info("Deleted S3 Access point", "bucket", bucket)
		return nil
	} else {
		if aerr, ok := err.(awserr.Error); ok {
			if aerr.Code() == "NoSuchAccessPoint" {
				return nil // Already deleted
			} else {
				return err
			}
		} else {
			return err
		}
	}
}

func (r *S3Reconciler) ReconcileS3RolePolicy(ctx context.Context,
	bucket *corev1alpha1.S3Bucket,
	status *corev1alpha1.S3BucketStatus,
	roleName string) error {

	log := log.FromContext(ctx)
	svc := iam.New(r.AWS.sess)

	if _, err := svc.GetRolePolicy(&iam.GetRolePolicyInput{
		PolicyName: &bucket.AccessPointName,
		RoleName:   &roleName,
	}); err == nil {
		status.RolePolicy = bucket.AccessPointName
		return nil // Policy exists.
	} else if aerr, ok := err.(awserr.Error); ok {
		if aerr.Code() == iam.ErrCodeNoSuchEntityException {
			// Policy does not exist. Continue.
		} else {
			return err
		}
	} else {
		return err
	}

	// Create policy.
	policyDoumentTemplate, err := os.ReadFile(
		"../templates/aws/policies/s3-policy.json")
	if err != nil {
		return err
	}
	tmpl, err := template.New("efs-policy").Parse(string(policyDoumentTemplate))
	if err != nil {
		return err
	}
	rolePolicyDocument := new(strings.Builder)
	if err = tmpl.Execute(rolePolicyDocument, map[string]any{
		"bucket": bucket.Name,
		"prefix": bucket.Path,
	}); err != nil {
		return err
	}
	if _, err := svc.PutRolePolicy(&iam.PutRolePolicyInput{
		PolicyDocument: aws.String(rolePolicyDocument.String()),
		PolicyName:     &bucket.AccessPointName,
		RoleName:       &roleName,
	}); err == nil {
		log.Info("S3 Role Policy created", "bucket", bucket)
		status.RolePolicy = bucket.AccessPointName
		return nil
	} else {
		return err
	}
}
