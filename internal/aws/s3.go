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
	"github.com/aws/aws-sdk-go/service/s3"
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

	status.AWS.S3.Buckets = make([]corev1alpha1.S3BucketStatus, 0,
		len(spec.AWS.S3.Buckets))

	for i, bucket := range spec.AWS.S3.Buckets {
		status.AWS.S3.Buckets = append(status.AWS.S3.Buckets,
			corev1alpha1.S3BucketStatus{
				Name: bucket.Name,
			})
		bucketStatus := &status.AWS.S3.Buckets[i]

		if err := r.ReconcileS3Path(ctx, &bucket,
			bucketStatus); err != nil {
			log.Error(err, "Failed reconciling S3 path", "bucket", bucket)
			return err
		}

		if err := r.ReconcileS3AccessPoint(ctx, &bucket,
			bucketStatus); err != nil {
			log.Error(err, "Failed reconciling S3 Access Point",
				"bucket", bucket)
			return err
		}

		if err := r.AttachPolicyToS3AccessPoint(ctx, &bucket,
			bucketStatus, status.AWS.Role.ARN); err != nil {
			log.Error(err, "Failed attaching S3 Access Point Policy",
				"bucket", bucket)
			return err
		}

	}
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

		if err := r.DeleteS3AccessPoint(ctx, &bucket,
			bucketStatus); err != nil {
			log.Error(err, "Failed to delete S3 Access Point", "bucket", bucket)
		}
	}
	status.AWS.S3.Buckets = bucketStatuses
	return nil
}

func (r *S3Reconciler) ReconcileS3Path(ctx context.Context,
	bucket *corev1alpha1.S3Bucket,
	status *corev1alpha1.S3BucketStatus) error {

	log := log.FromContext(ctx)

	svc := s3.New(r.AWS.sess)

	if _, err := svc.HeadObject(&s3.HeadObjectInput{
		Bucket: aws.String(bucket.Name),
		Key:    aws.String(bucket.Path),
	}); err != nil {
		if aerr, ok := err.(awserr.Error); ok && aerr.Code() == "NotFound" {
			// Path does not exist. Create it.
			_, err = svc.PutObject(&s3.PutObjectInput{
				Bucket: aws.String(bucket.Name),
				Key:    aws.String(bucket.Path),
			})
			if err != nil {
				return err
			}
			log.Info("Created S3 path", "bucket", bucket.Name, "path", bucket.Path)
		} else {
			return err
		}
	}

	status.Path = bucket.Path
	return nil
}

func (r *S3Reconciler) ReconcileS3AccessPoint(ctx context.Context,
	bucket *corev1alpha1.S3Bucket,
	status *corev1alpha1.S3BucketStatus) error {

	log := log.FromContext(ctx)
	svc := s3control.New(r.AWS.sess)

	if ap, err := svc.GetAccessPoint(&s3control.GetAccessPointInput{
		AccountId: aws.String(r.AWS.config.AccountID),
		Name:      aws.String(strings.ToLower(bucket.AccessPointName)),
	}); err == nil {
		// Access point exists.
		status.AccessPointARN = *ap.AccessPointArn
	} else if aerr, ok := err.(awserr.Error); ok {
		if aerr.Code() == "NoSuchAccessPoint" {
			// Access point does not exist. Create it.
			if ap, err := svc.CreateAccessPoint(&s3control.CreateAccessPointInput{
				AccountId: aws.String(r.AWS.config.AccountID),
				Bucket:    aws.String(bucket.Name),
				Name:      aws.String(strings.ToLower(bucket.AccessPointName)),
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
		Name:      aws.String(strings.ToLower(bucket.AccessPointName)),
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

func (r *S3Reconciler) AttachPolicyToS3AccessPoint(ctx context.Context,
	bucket *corev1alpha1.S3Bucket,
	status *corev1alpha1.S3BucketStatus,
	roleARN string) error {

	log := log.FromContext(ctx)
	svc := s3control.New(r.AWS.sess)

	if _, err := svc.GetAccessPointPolicy(&s3control.GetAccessPointPolicyInput{
		AccountId: aws.String(r.AWS.config.AccountID),
		Name:      aws.String(strings.ToLower(bucket.AccessPointName)),
	}); err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			if aerr.Code() == "NoSuchAccessPointPolicy" {
				if roleARN == "" {
					return nil // No role ARN to attach.
				}
				// Access point policy does not exist. Create it.
				policyDoumentTemplate, err := os.ReadFile(
					"../templates/aws/policies/s3-policy.json")
				if err != nil {
					return err
				}
				tmpl, err := template.New("s3-policy").Parse(
					string(policyDoumentTemplate))
				if err != nil {
					return err
				}
				rolePolicyDocument := new(strings.Builder)
				if err = tmpl.Execute(rolePolicyDocument, map[string]any{
					"roleARN":        roleARN,
					"accessPointARN": status.AccessPointARN,
					"prefix":         bucket.Path,
				}); err != nil {
					return err
				}

				if _, err := svc.PutAccessPointPolicy(
					&s3control.PutAccessPointPolicyInput{
						AccountId: aws.String(r.AWS.config.AccountID),
						Name: aws.String(strings.ToLower(
							bucket.AccessPointName)),
						Policy: aws.String(rolePolicyDocument.String()),
					}); err != nil {
					return err
				}
				log.Info("Attached policy to S3 Access point", "bucket", bucket)
			} else {
				return err
			}
		} else {
			return err
		}
	}
	return nil
}
