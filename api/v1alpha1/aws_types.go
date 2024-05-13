package v1alpha1


type EFSSpec struct {
	RootDirectory string    `json:"rootDirectory,omitempty"`
}

type EFSStatus struct {
	AccessPointID string `json:"accessPointID,omitempty"`
}

