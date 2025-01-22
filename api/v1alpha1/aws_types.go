package v1alpha1

type AWSSpec struct {
	RoleName string  `json:"roleName,omitempty"`
	EFS      EFSSpec `json:"efs,omitempty"`
	S3       S3Spec  `json:"s3,omitempty"`
}

type EFSSpec struct {
	AccessPoints []EFSAccess `json:"accessPoints,omitempty"`
}

type EFSAccess struct {
	Name          string `json:"name,omitempty"`
	FSID          string `json:"fsID,omitempty"`
	AccessPointID string `json:"accessPointID,omitempty"`
	RootDirectory string `json:"rootDirectory,omitempty"`
	User          User   `json:"user,omitempty"`
	Permissions   string `json:"permissions,omitempty"`
}

type S3Spec struct {
	Buckets []S3Bucket `json:"buckets,omitempty"`
}

type S3Bucket struct {
	Name            string `json:"name,omitempty"`
	Path            string `json:"path,omitempty"`
	AccessPointName string `json:"accessPointName,omitempty"`
	EnvVar          string `json:"envVar,omitempty"`
}

type AWSStatus struct {
	Role AWSRoleStatus `json:"role,omitempty"`
	EFS  EFSStatus     `json:"efs,omitempty"`
	S3   S3Status      `json:"s3,omitempty"`
}

type AWSRoleStatus struct {
	Name string `json:"name,omitempty"`
	ARN  string `json:"arn,omitempty"`
}

type EFSStatus struct {
	AccessPoints []EFSAccessStatus `json:"accessPoints,omitempty"`
}

type EFSAccessStatus struct {
	Name          string `json:"name,omitempty"`
	AccessPointID string `json:"accessPointID,omitempty"`
	FSID          string `json:"fsID,omitempty"`
}

type S3Status struct {
	Buckets []S3BucketStatus `json:"buckets,omitempty"`
}

type S3BucketStatus struct {
	Name           string `json:"name,omitempty"`
	AccessPointARN string `json:"accessPointARN,omitempty"`
	RolePolicy     string `json:"rolePolicy,omitempty"`
	Path           string `json:"path,omitempty"`
	EnvVar         string `json:"envVar,omitempty"`
}
