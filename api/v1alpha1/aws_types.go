package v1alpha1

type EFSSpec struct {
	RootDirectory string `json:"rootDirectory,omitempty"`
}

type EFSStatus struct {
	AccessPointID string `json:"accessPointID,omitempty"`
}

type S3Bucket struct {
	Name   string `json:"name,omitempty"`
	Create bool   `json:"create,omitempty"`
	Path   string `json:"path,omitempty"`
}

type S3Spec struct {
	Buckets []S3Bucket `json:"buckets,omitempty"`
}

type S3BucketStatus struct {
	Name          string `json:"name,omitempty"`
	AccessPointID string `json:"accessPointID,omitempty"`
	RolePolicyID  string `json:"rolePolicyID,omitempty"`
}

type S3Status struct {
	Buckets []S3BucketStatus `json:"buckets,omitempty"`
}
