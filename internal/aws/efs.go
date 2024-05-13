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

	"github.com/UKEODHP/workspace-controller/api/v1alpha1"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/efs"
	"github.com/google/uuid"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func (c *AWSClient) ReconcileEFSAccessPoint(ctx context.Context, efsID string,
	awsEFS *v1alpha1.EFSSpec, user efs.PosixUser) (*string, error) {

	log := log.FromContext(ctx)

	// Create a new EFS service client
	svc := efs.New(c.sess)

	// Get the access point
	describeAccessPointsParams := &efs.DescribeAccessPointsInput{
		FileSystemId: aws.String(efsID),
	}
	accessPoints, err := svc.DescribeAccessPoints(describeAccessPointsParams)
	if err != nil {
		log.Error(err, "Failed to describe EFS access points", "file system ID", efsID)
		return nil, err
	}

	// Find the access point with the desired root directory
	var accessPointID string
	for _, ap := range accessPoints.AccessPoints {
		if aws.StringValue(ap.RootDirectory.Path) == awsEFS.RootDirectory {
			accessPointID = aws.StringValue(ap.AccessPointId)
			break
		}
	}

	// If the access point is found, return its ID
	if accessPointID != "" {
		return &accessPointID, nil
	}

	// Access point not found, create a new one
	// Define the parameters for the access point
	accessPointParams := &efs.CreateAccessPointInput{
		ClientToken:  aws.String(uuid.New().String()),
		FileSystemId: aws.String(efsID),
		PosixUser:    &user,
		RootDirectory: &efs.RootDirectory{
			Path: aws.String(awsEFS.RootDirectory),
			CreationInfo: &efs.CreationInfo{
				OwnerUid:    aws.Int64(*user.Uid),
				OwnerGid:    aws.Int64(*user.Gid),
				Permissions: aws.String("755"),
			},
		},
	}

	// Create the access point
	if ap, err := svc.CreateAccessPoint(accessPointParams); err == nil {
		log.Info("Created EFS access point", "access point", accessPointParams)
		return ap.AccessPointId, nil
	} else {
		log.Error(err, "Failed to create EFS access point", "access point", accessPointParams)
		return nil, err
	}
}

func (c *AWSClient) DeleteEFSAccessPoint(ctx context.Context,
	accessPointID string) error {

	log := log.FromContext(ctx)

	// Create a new EFS service client
	svc := efs.New(c.sess)

	// Delete the access point
	deleteAccessPointParams := &efs.DeleteAccessPointInput{
		AccessPointId: aws.String(accessPointID),
	}

	if _, err := svc.DeleteAccessPoint(deleteAccessPointParams); err == nil {
		log.Info("Deleted EFS access point", "access point ID", accessPointID)
		return err
	}

	return nil
}
