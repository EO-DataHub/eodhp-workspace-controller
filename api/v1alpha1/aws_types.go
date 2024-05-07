package v1alpha1

type PosixUser struct {
	UID int64 `json:"uid,omitempty"`
	GID int64 `json:"gid,omitempty"`
}
type AWSEFSSpec struct {
	RootDirectory string    `json:"rootDirectory,omitempty"`
	PosixUser     PosixUser `json:"user,omitempty"`
}

type AWSEFSStatus struct {
	AccessPointID string `json:"accessPointID,omitempty"`
}
