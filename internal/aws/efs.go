package aws

import (
	"context"

	"github.com/UKEODHP/workspace-controller/api/v1alpha1"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/efs"
	"github.com/google/uuid"
)

func (c *AWSClient) ReconcileEFSAccessPoint(ctx context.Context, efsID string,
	awsEFS *v1alpha1.AWSEFSSpec) (*string, error) {

	// Create a new EFS service client
	svc := efs.New(c.sess)

	// Define the parameters for the access point
	var uid, gid int64
	if awsEFS.PosixUser.UID == 0 {
		uid = 1000
	} else {
		uid = awsEFS.PosixUser.UID
	}
	if awsEFS.PosixUser.GID == 0 {
		gid = 1000
	} else {
		gid = awsEFS.PosixUser.GID
	}
	accessPointParams := &efs.CreateAccessPointInput{
		ClientToken:  aws.String(uuid.New().String()),
		FileSystemId: aws.String(efsID),
		PosixUser: &efs.PosixUser{
			Uid: aws.Int64(uid),
			Gid: aws.Int64(gid),
		},
		RootDirectory: &efs.RootDirectory{
			Path: aws.String(awsEFS.RootDirectory),
		},
	}

	// Create the access point
	if ap, err := svc.CreateAccessPoint(accessPointParams); err == nil {
		return ap.AccessPointId, nil
	} else {
		return nil, err
	}
}

func (c *AWSClient) DeleteEFSAccessPoint(ctx context.Context,
	accessPointID string) error {

	// Create a new EFS service client
	svc := efs.New(c.sess)

	// Delete the access point
	deleteAccessPointParams := &efs.DeleteAccessPointInput{
		AccessPointId: aws.String(accessPointID),
	}
	_, err := svc.DeleteAccessPoint(deleteAccessPointParams)
	if err != nil {
		return err
	}

	return nil
}
