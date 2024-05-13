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
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3control"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func (c *AWSClient) ReconcileS3Bucket(ctx context.Context, bucket corev1alpha1.S3Bucket) error {

	log := log.FromContext(ctx)

	svc := s3.New(c.sess)

	if _, err := svc.HeadBucket(&s3.HeadBucketInput{
		Bucket: &bucket.Name,
	}); err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case s3.ErrCodeNoSuchBucket:
				// Bucket does not exist.
				if bucket.Create {
					if _, err := svc.CreateBucket(&s3.CreateBucketInput{
						Bucket: &bucket.Name,
					}); err == nil {
						log.Info("Created S3 Bucket", "Bucket", bucket)
					} else {
						return err
					}
				} else {
					log.Error(err, "S3 Bucket does not exist and create disabled.",
						"bucket", bucket)
					return err
				}
			default:
				return err
			}
		} else {
			return err
		}
	}

	if _, err := svc.HeadObject(&s3.HeadObjectInput{
		Bucket: &bucket.Name,
		Key:    &bucket.Path,
	}); err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case s3.ErrCodeNoSuchKey:
				// Directory does not exist. Create it.
				_, err := svc.PutObject(&s3.PutObjectInput{
					Bucket: &bucket.Name,
					Key:    &bucket.Path,
				})
				if err == nil {
					log.Info("Created S3 Bucket directory", "Bucket", bucket)
				} else {
					return err
				}
			default:
				return err
			}
		} else {
			return err
		}
	}

	return nil
}

func (c *AWSClient) ReconcileS3AccessPoint(ctx context.Context, bucketName,
	accessPointName string) (*string, error) {

	log := log.FromContext(ctx)
	svc := s3control.New(c.sess)

	if ap, err := svc.GetAccessPoint(&s3control.GetAccessPointInput{
		AccountId: aws.String(c.config.AccountID),
		Name:      aws.String(accessPointName),
	}); err == nil {
		// Access point exists.
		log.Info("Access point already exists", "access point", accessPointName)
		apID := ap.String()
		return &apID, nil
	} else if aerr, ok := err.(awserr.Error); ok {
		if aerr.Code() == s3control.ErrCodeNotFoundException {
			// Access point does not exist. Create it.
			if ap, err := svc.CreateAccessPoint(&s3control.CreateAccessPointInput{
				AccountId: aws.String(c.config.AccountID),
				Bucket:    aws.String(bucketName),
				Name:      aws.String(accessPointName),
			}); err == nil {
				log.Info("Created S3 Access point", "bucket name", bucketName,
					"access point", accessPointName)
				apID := ap.String()
				return &apID, nil
			} else {
				return nil, err
			}
		} else {
			return nil, err
		}
	} else {
		return nil, err
	}
}

func (c *AWSClient) DeleteS3AccessPoint(ctx context.Context, accessPointName string) error {

	log := log.FromContext(ctx)
	svc := s3control.New(c.sess)

	if _, err := svc.DeleteAccessPoint(&s3control.DeleteAccessPointInput{
		AccountId: aws.String(c.config.AccountID),
		Name:      aws.String(accessPointName),
	}); err != nil {
		log.Error(err, "Failed deleting S3 Access point", "access point", accessPointName)
		return err
	}
	log.Info("Deleted S3 Access point", "access point", accessPointName)
	return nil
}

func (c *AWSClient) ReconcileS3RolePolicy(ctx context.Context, policyName string,
	bucket corev1alpha1.S3Bucket, role *iam.Role) (*string, error) {

	log := log.FromContext(ctx)
	svc := iam.New(c.sess)

	if policy, err := svc.GetRolePolicy(&iam.GetRolePolicyInput{
		PolicyName: &policyName,
		RoleName:   role.RoleName,
	}); err == nil {
		return policy.PolicyDocument, nil // Policy exists.
	} else if aerr, ok := err.(awserr.Error); ok {
		if aerr.Code() == iam.ErrCodeNoSuchEntityException {
			// Policy does not exist. Continue.
			log.Info("Policy does not exist", "policy", policyName, "role", role.RoleName)
		} else {
			return nil, err
		}
	} else {
		return nil, err
	}

	// Create policy.
	policyDoumentTemplate, err := os.ReadFile("../templates/aws/policies/s3-policy.json")
	if err != nil {
		return nil, err
	}
	tmpl, err := template.New("efs-policy").Parse(string(policyDoumentTemplate))
	if err != nil {
		return nil, err
	}
	rolePolicyDocument := new(strings.Builder)
	if err = tmpl.Execute(rolePolicyDocument, map[string]any{
		"accountID": c.config.AccountID,
		"region":    c.config.Region,
		"bucket":    bucket.Name,
		"prefix":    bucket.Path,
	}); err != nil {
		return nil, err
	}
	if policy, err := svc.PutRolePolicy(&iam.PutRolePolicyInput{
		PolicyDocument: aws.String(rolePolicyDocument.String()),
		PolicyName:     &policyName,
		RoleName:       role.RoleName,
	}); err == nil {
		log.Info("Policy created", "policy", policyName, "role", role.RoleName)
		p := policy.String()
		return &p, nil
	} else {
		return nil, err
	}
}
