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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type IAMRoleReconciler struct {
	client.Client
	AWS AWSClient
}

func (r *IAMRoleReconciler) Reconcile(ctx context.Context,
	spec *corev1alpha1.WorkspaceSpec,
	status *corev1alpha1.WorkspaceStatus) error {

	log := log.FromContext(ctx)
	svc := iam.New(r.AWS.sess)

	if role, err := svc.GetRole(&iam.GetRoleInput{
		RoleName: &spec.AWS.RoleName,
	}); err == nil {
		status.AWS.Role.Name = *role.Role.RoleName
		status.AWS.Role.ARN = *role.Role.Arn
		return nil // Role exists.
	} else if aerr, ok := err.(awserr.Error); ok {
		if aerr.Code() == iam.ErrCodeNoSuchEntityException {
			// Role does not exist. Continue.
		} else {
			return err
		}
	} else {
		return err
	}

	// Create role.
	trustPolicy, err := os.ReadFile(
		"../templates/aws/policies/trust-policy.json",
	)
	if err != nil {
		return err
	}
	tmpl, err := template.New("trust-policy").Parse(string(trustPolicy))
	if err != nil {
		return err
	}
	assumeRolePolicyDocument := new(strings.Builder)
	if err := tmpl.Execute(assumeRolePolicyDocument, map[string]any{
		"accountID": r.AWS.config.AccountID,
		"oidc": map[string]any{
			"provider": r.AWS.config.OIDC.Provider,
		},
		"namespace":      spec.Namespace,
		"serviceAccount": spec.ServiceAccount.Name,
	}); err != nil {
		return err
	}
	if role, err := svc.CreateRole(&iam.CreateRoleInput{
		RoleName:                 &spec.AWS.RoleName,
		AssumeRolePolicyDocument: aws.String(assumeRolePolicyDocument.String()),
	}); err == nil {
		log.Info("Role created", "RoleName", role.Role.RoleName)
		status.AWS.Role.Name = *role.Role.RoleName
		status.AWS.Role.ARN = *role.Role.Arn
		return nil
	} else {
		return err
	}
}

func (r *IAMRoleReconciler) Teardown(ctx context.Context,
	spec *corev1alpha1.WorkspaceSpec,
	status *corev1alpha1.WorkspaceStatus) error {

	log := log.FromContext(ctx)
	svc := iam.New(r.AWS.sess)

	if status.AWS.Role.Name != "" {
		// Delete all policies attached to the role.
		listAttachedPoliciesOutput, err := svc.ListAttachedRolePolicies(&iam.ListAttachedRolePoliciesInput{
			RoleName: aws.String(status.AWS.Role.Name),
		})
		if err != nil {
			if aerr, ok := err.(awserr.Error); ok {
				if aerr.Code() == iam.ErrCodeNoSuchEntityException {
					return nil // Role doesn't exist.
				} else {
					return err
				}
			}
		}
		for _, policy := range listAttachedPoliciesOutput.AttachedPolicies {
			_, err := svc.DetachRolePolicy(&iam.DetachRolePolicyInput{
				RoleName:  aws.String(status.AWS.Role.Name),
				PolicyArn: policy.PolicyArn,
			})
			if err != nil {
				return err
			}
		}
		// Delete the IAM role
		if _, err := svc.DeleteRole(&iam.DeleteRoleInput{
			RoleName: aws.String(status.AWS.Role.Name),
		}); err == nil {
			log.Info("Role deleted", "Role", status.AWS.Role.Name)
			status.AWS.Role.Name = ""
			return nil
		} else {
			if aerr, ok := err.(awserr.Error); ok {
				if aerr.Code() == iam.ErrCodeNoSuchEntityException {
					return nil // Role doesn't exist.
				} else {
					return err
				}
			}
			return err
		}
	} else {
		return nil // Status doesn't have any role name.
	}
}
