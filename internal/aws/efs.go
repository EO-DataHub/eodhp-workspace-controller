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
	"github.com/aws/aws-sdk-go/service/efs"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/google/uuid"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type EFSReconciler struct {
	client.Client
	AWS AWSClient
}

func (r *EFSReconciler) Reconcile(ctx context.Context,
	spec *corev1alpha1.WorkspaceSpec,
	status *corev1alpha1.WorkspaceStatus) error {

	log := log.FromContext(ctx)
	efsStatuses := make(
		[]corev1alpha1.EFSAccessStatus, 0,
		len(spec.AWS.EFS.AccessPoints),
	)
	for _, efsAccess := range spec.AWS.EFS.AccessPoints {
		efsStatus := corev1alpha1.EFSAccessStatus{
			Name: efsAccess.Name,
		}
		if accessPointID, err := r.ReconcileEFSAccessPoint(ctx,
			efsAccess); err == nil {
			log.Info("EFS access point reconciled",
				"access point ID", accessPointID)
			efsStatus.AccessPointID = *accessPointID
		} else {
			log.Error(err, "Failed to reconcile EFS access point")
			return err
		}
		if status.AWS.RoleName != "" {
			if err := r.ReconcileEFSRolePolicy(ctx, efsAccess,
				status.AWS.RoleName); err != nil {
				log.Error(err, "Failed to reconcile EFS role policy",
					"policy", efsAccess.Name, "role", status.AWS.RoleName)
				return err
			}

		}
		efsStatuses = append(efsStatuses, efsStatus)
	}

	status.AWS.EFS.AccessPoints = efsStatuses
	return nil
}

func (r *EFSReconciler) Teardown(ctx context.Context,
	spec *corev1alpha1.WorkspaceSpec,
	status *corev1alpha1.WorkspaceStatus) error {

	log := log.FromContext(ctx)
	for _, efsStatus := range status.AWS.EFS.AccessPoints {
		if status.AWS.RoleName != "" {
			svc := iam.New(r.AWS.sess)
			if _, err := svc.DeleteRolePolicy(&iam.DeleteRolePolicyInput{
				PolicyName: &efsStatus.Name, RoleName: &status.AWS.RoleName,
			}); err == nil {
				log.Info("Deleted EFS Role Policy", "policy", &efsStatus.Name,
					"role", &status.AWS.RoleName)
			} else {
				log.Error(err, "Failed to delete EFS Role Policy",
					"policy", &efsStatus.Name, "role", &status.AWS.RoleName)
			}

		}
		if err := r.DeleteEFSAccessPoint(ctx,
			efsStatus.AccessPointID); err == nil {
			log.Info("Deleted EFS Access Point",
				"access point ID", efsStatus.AccessPointID)
		} else {
			log.Info("Failed to delete EFS Access Point",
				"access point ID", efsStatus.AccessPointID)
		}
	}

	status.AWS.EFS.AccessPoints = make([]corev1alpha1.EFSAccessStatus, 0)
	return nil
}

func (r *EFSReconciler) ReconcileEFSAccessPoint(ctx context.Context,
	efsAccess corev1alpha1.EFSAccess) (*string, error) {

	log := log.FromContext(ctx)

	// Create a new EFS service client
	svc := efs.New(r.AWS.sess)

	// Get the access point
	describeAccessPointsParams := &efs.DescribeAccessPointsInput{
		FileSystemId: aws.String(efsAccess.FSID),
	}
	accessPoints, err := svc.DescribeAccessPoints(describeAccessPointsParams)
	if err != nil {
		return nil, err
	}

	// Find the access point with the desired root directory
	var accessPointID string
	for _, ap := range accessPoints.AccessPoints {
		if aws.StringValue(ap.RootDirectory.Path) == efsAccess.RootDirectory {
			accessPointID = aws.StringValue(ap.AccessPointId)
			break
		}
	}

	if accessPointID != "" {
		// Access point has been found
		return &accessPointID, nil
	}

	// Access point not found, create a new one
	// Create the access point
	if ap, err := svc.CreateAccessPoint(&efs.CreateAccessPointInput{
		ClientToken:  aws.String(uuid.New().String()),
		FileSystemId: aws.String(efsAccess.FSID),
		PosixUser: &efs.PosixUser{
			Uid: aws.Int64(efsAccess.User.UID),
			Gid: aws.Int64(efsAccess.User.GID),
		},
		RootDirectory: &efs.RootDirectory{
			Path: aws.String(efsAccess.RootDirectory),
			CreationInfo: &efs.CreationInfo{
				OwnerUid:    aws.Int64(efsAccess.User.UID),
				OwnerGid:    aws.Int64(efsAccess.User.GID),
				Permissions: aws.String(efsAccess.Permissions),
			},
		},
	}); err == nil {
		log.Info("Created EFS access point", "access point", ap.AccessPointId)
		return ap.AccessPointId, nil
	} else {
		return nil, err
	}
}

func (r *EFSReconciler) DeleteEFSAccessPoint(ctx context.Context,
	accessPointID string) error {

	log := log.FromContext(ctx)

	// Create a new EFS service client
	svc := efs.New(r.AWS.sess)

	// Delete the access point
	if _, err := svc.DeleteAccessPoint(&efs.DeleteAccessPointInput{
		AccessPointId: aws.String(accessPointID),
	}); err == nil {
		log.Info("Deleted EFS access point", "access point ID", accessPointID)
		return nil
	} else {
		return err
	}
}

func (r *EFSReconciler) ReconcileEFSRolePolicy(ctx context.Context,
	efsAccess corev1alpha1.EFSAccess, roleName string) error {

	log := log.FromContext(ctx)
	svc := iam.New(r.AWS.sess)

	if _, err := svc.GetRolePolicy(&iam.GetRolePolicyInput{
		PolicyName: &efsAccess.Name,
		RoleName:   &roleName,
	}); err == nil {
		return nil // Policy exists.
	} else if aerr, ok := err.(awserr.Error); ok {
		if aerr.Code() != iam.ErrCodeNoSuchEntityException {
			return err
		}
	} else {
		return err
	}

	// Create policy.
	policyDoumentTemplate, err := os.ReadFile(
		"../templates/aws/policies/efs-policy.json")
	if err != nil {
		return err
	}
	tmpl, err := template.New("efs-policy").Parse(string(policyDoumentTemplate))
	if err != nil {
		return err
	}
	rolePolicyDocument := new(strings.Builder)
	if err = tmpl.Execute(rolePolicyDocument, map[string]any{
		"accountID": r.AWS.config.AccountID,
		"region":    r.AWS.config.Region,
		"efsID":     efsAccess.FSID,
	}); err != nil {
		return err
	}
	if _, err := svc.PutRolePolicy(&iam.PutRolePolicyInput{
		PolicyDocument: aws.String(rolePolicyDocument.String()),
		PolicyName:     &efsAccess.Name,
		RoleName:       &roleName,
	}); err == nil {
		log.Info("Policy created", "policy", efsAccess.Name, "role", roleName)
		return nil
	} else {
		return err
	}
}
