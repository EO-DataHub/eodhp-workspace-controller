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

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"sigs.k8s.io/controller-runtime/pkg/log"

	corev1alpha1 "github.com/UKEODHP/workspace-controller/api/v1alpha1"
)

type AWSConfig struct {
	AccountID string `yaml:"accountID"`
	Region    string `yaml:"region"`
	Auth      struct {
		AccessKey string `yaml:"accessKey"`
		SecretKey string `yaml:"secretKey"`
	} `yaml:"auth"`
	OIDC struct {
		Provider string `yaml:"provider"`
	}
	Storage struct {
		EFSID string `yaml:"efsID"`
	}
	UniqueString string `yaml:"uniqueString"`
}

type AWSClient struct {
	config AWSConfig
	sess   *session.Session
}

func (c *AWSClient) Initialise(config AWSConfig) error {
	c.config = config

	// Create a new session.
	if sess, err := session.NewSession(&aws.Config{
		Region: aws.String(config.Region),
	}); err == nil {
		c.sess = sess
		return nil
	} else {
		fmt.Println("Error creating AWS session ", err)
		return err
	}
}

func (c *AWSClient) Enabled() bool {
	return c.sess != nil
}

func (c *AWSClient) Reconcile(ctx context.Context, workspace *corev1alpha1.Workspace) error {
	log := log.FromContext(ctx)
	uniqueName := fmt.Sprintf("%s-%s", workspace.Name, c.config.UniqueString)

	if c.Enabled() {
		role, err := c.ReconcileIAMRole(ctx, uniqueName, workspace.Status.Namespace)
		if err != nil {
			log.Error(err, "Failed to reconcile IAM role", "role", workspace.Name)
			return err
		}
		workspace.Status.AWSRole = *role.RoleName
		if _, err = c.ReconcileIAMRolePolicy(ctx, uniqueName, role); err != nil {
			log.Error(err, "Failed to reconcile IAM role policy", "role", workspace.Name)
			return err
		}
	}
	return nil
}

func (c *AWSClient) DeleteChildResources(ctx context.Context, workspace *corev1alpha1.Workspace) error {
	log := log.FromContext(ctx)
	uniqueName := fmt.Sprintf("%s-%s", workspace.Name, c.config.UniqueString)

	if c.Enabled() {
		if err := c.DeleteIAMRolePolicy(ctx, uniqueName); err != nil {
			log.Error(err, "Failed to delete IAM role policy", "role", uniqueName)
			return err
		}
		if err := c.DeleteIAMRole(ctx, uniqueName); err != nil {
			log.Error(err, "Failed to delete IAM role", "role", uniqueName)
			return err
		}
	}
	return nil
}
