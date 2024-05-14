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
	"fmt"
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
	AWS        AWSClient
	RoleSuffix string
}

func (r *IAMRoleReconciler) Reconcile(ctx context.Context,
	ws *corev1alpha1.Workspace) error {

	log := log.FromContext(ctx)
	svc := iam.New(r.AWS.sess)

	roleName := fmt.Sprintf("%s%s", ws.Name, r.RoleSuffix)
	if role, err := svc.GetRole(&iam.GetRoleInput{
		RoleName: &roleName,
	}); err == nil {
		ws.Status.AWSRole = *role.Role.RoleName
		if err := r.Update(ctx, ws); err != nil {
			log.Error(err, "Failed to update Workspace Status", "AWS Role", role)
		}
		return nil // Role exists.
	} else if aerr, ok := err.(awserr.Error); ok {
		if aerr.Code() == iam.ErrCodeNoSuchEntityException {
			// Role does not exist. Continue.
			log.Info("Role does not exist", "Role", roleName, "Namespace", ws.Spec.Namespace)
		} else {
			return err
		}
	} else {
		return err
	}

	// Create role.
	trustPolicy, err := os.ReadFile("../templates/aws/policies/trust-policy.json")
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
		"namespace":      ws.Spec.Namespace,
		"serviceAccount": "workspace-controller",
	}); err != nil {
		return err
	}
	if role, err := svc.CreateRole(&iam.CreateRoleInput{
		RoleName:                 &roleName,
		Path:                     aws.String("/"),
		AssumeRolePolicyDocument: aws.String(assumeRolePolicyDocument.String()),
	}); err == nil {
		log.Info("Role created", "Role", role)
		ws.Status.AWSRole = *role.Role.RoleName
		if err := r.Update(ctx, ws); err != nil {
			log.Error(err, "Failed to update Workspace Status", "AWS Role", role)
		}
		return nil
	} else {
		return err
	}
}

func (r *IAMRoleReconciler) Teardown(ctx context.Context,
	ws *corev1alpha1.Workspace) error {

	log := log.FromContext(ctx)
	svc := iam.New(r.AWS.sess)

	if ws.Status.AWSRole != "" {
		// Delete the IAM role
		if _, err := svc.DeleteRole(&iam.DeleteRoleInput{
			RoleName: aws.String(ws.Status.AWSRole),
		}); err == nil {
			log.Info("Role deleted", "Role", ws.Status.AWSRole)
			ws.Status.AWSRole = ""
			if err := r.Update(ctx, ws); err != nil {
				log.Error(err, "Failed to update Workspace Status",
					"AWS Role", ws.Status.AWSRole)
			}
		} else {
			if aerr, ok := err.(awserr.Error); ok {
				if aerr.Code() == iam.ErrCodeNoSuchEntityException {
					return nil
				} else {
					return err
				}
			}
			return err
		}
	}
	return nil
}
