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

package controller

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	"text/template"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/iam"
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

type awsClient struct {
	config AWSConfig
	sess   *session.Session
}

var lock = &sync.Mutex{}
var awsClient_ *awsClient

func GetAWSClient() *awsClient {
	if awsClient_ == nil {
		lock.Lock()
		defer lock.Unlock()
		if awsClient_ == nil {
			awsClient_ = &awsClient{}
		}
	}
	return awsClient_
}

func (c *awsClient) Initialise(config AWSConfig) error {
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

func (c *awsClient) Enabled() bool {
	return c.sess != nil
}

func (c *awsClient) Reconcile(ctx context.Context, workspace *corev1alpha1.Workspace) error {
	log := log.FromContext(ctx)
	uniqueName := fmt.Sprintf("%s-%s", workspace.Name, c.config.UniqueString)

	if c.Enabled() {
		role, err := c.ReconcileIAMRole(ctx, uniqueName, workspace.Status.Namespace)
		if err != nil {
			log.Error(err, "Failed to reconcile IAM role", "role", workspace.Name)
			return err
		}
		if _, err = c.ReconcileIAMRolePolicy(ctx, uniqueName, role); err != nil {
			log.Error(err, "Failed to reconcile IAM role policy", "role", workspace.Name)
			return err
		}
	}
	return nil
}

func (c *awsClient) ReconcileIAMUser(username string) (*iam.User, error) {
	iam_ := iam.New(c.sess)

	if user, err := iam_.GetUser(&iam.GetUserInput{
		UserName: &username,
	}); err == nil {
		return user.User, nil // User exists.
	} else if aerr, ok := err.(awserr.Error); ok {
		if aerr.Code() != iam.ErrCodeNoSuchEntityException {
			return nil, err
		}
	} else {
		return nil, err
	}

	// User does not exist. Create user.
	if user, err := iam_.CreateUser(&iam.CreateUserInput{
		UserName: &username,
	}); err == nil {
		return user.User, nil
	} else {
		return nil, err
	}
}

func (c *awsClient) ReconcileIAMRole(ctx context.Context, roleName, namespace string) (*iam.Role, error) {
	log := log.FromContext(ctx)

	iam_ := iam.New(c.sess)

	if role, err := iam_.GetRole(&iam.GetRoleInput{
		RoleName: &roleName,
	}); err == nil {
		return role.Role, nil // Role exists.
	} else if aerr, ok := err.(awserr.Error); ok {
		if aerr.Code() == iam.ErrCodeNoSuchEntityException {
			// Role does not exist. Continue.
			log.Info("Role does not exist", "role", roleName, "namespace", namespace)
		} else {
			return nil, err
		}
	} else {
		return nil, err
	}

	// Create role.
	trustPolicy, err := os.ReadFile("../templates/aws/policies/trust-policy.json")
	if err != nil {
		return nil, err
	}
	tmpl, err := template.New("trust-policy").Parse(string(trustPolicy))
	if err != nil {
		return nil, err
	}
	assumeRolePolicyDocument := new(strings.Builder)
	if err := tmpl.Execute(assumeRolePolicyDocument, map[string]any{
		"accountID": c.config.AccountID,
		"oidc": map[string]any{
			"provider": c.config.OIDC.Provider,
		},
		"namespace":      namespace,
		"serviceAccount": "workspace-controller",
	}); err != nil {
		return nil, err
	}
	if role, err := iam_.CreateRole(&iam.CreateRoleInput{
		RoleName:                 &roleName,
		Path:                     aws.String("/"),
		AssumeRolePolicyDocument: aws.String(assumeRolePolicyDocument.String()),
	}); err == nil {
		log.Info("Role created", "role", roleName, "namespace", namespace)
		return role.Role, nil
	} else {
		return nil, err
	}
}

func (c *awsClient) ReconcileIAMRolePolicy(ctx context.Context, policyName string, role *iam.Role) (*string, error) {
	log := log.FromContext(ctx)

	iam_ := iam.New(c.sess)

	if policy, err := iam_.GetRolePolicy(&iam.GetRolePolicyInput{
		PolicyName: &policyName,
		RoleName:   role.RoleName,
	}); err == nil {
		return policy.PolicyDocument, nil // Policy exists.
	} else if aerr, ok := err.(awserr.Error); ok {
		if aerr.Code() == iam.ErrCodeNoSuchEntityException {
			// Policy does not exist. Continue.
			log.Info("Policy does not exist", "policy", policyName, "role", role.RoleName)
		} else {
			return nil, err
		}
	} else {
		return nil, err
	}

	// Create policy.
	policyDoumentTemplate, err := os.ReadFile("../templates/aws/policies/efs-role-policy.json")
	if err != nil {
		return nil, err
	}
	tmpl, err := template.New("efs-role-policy").Parse(string(policyDoumentTemplate))
	if err != nil {
		return nil, err
	}
	rolePolicyDocument := new(strings.Builder)
	if err = tmpl.Execute(rolePolicyDocument, map[string]any{
		"accountID": c.config.AccountID,
		"efsID":     c.config.Storage.EFSID,
	}); err != nil {
		return nil, err
	}
	if policy, err := iam_.PutRolePolicy(&iam.PutRolePolicyInput{
		PolicyDocument: aws.String(rolePolicyDocument.String()),
		PolicyName:     &policyName,
		RoleName:       role.RoleName,
	}); err == nil {
		log.Info("Policy created", "policy", policyName, "role", role.RoleName)
		p := policy.String()
		return &p, nil
	} else {
		return nil, err
	}
}

func (c *awsClient) DeleteChildResources(ctx context.Context, workspace *corev1alpha1.Workspace) error {
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

func (c *awsClient) DeleteIAMRole(ctx context.Context, roleName string) error {
	log := log.FromContext(ctx)

	iam_ := iam.New(c.sess)

	if _, err := iam_.DeleteRole(&iam.DeleteRoleInput{
		RoleName: &roleName,
	}); err == nil {
		log.Info("Role deleted", "role", roleName)
		return nil
	} else if aerr, ok := err.(awserr.Error); ok {
		if aerr.Code() == iam.ErrCodeNoSuchEntityException {
			// Role does not exist. Continue.
			log.Info("Role does not exist", "role", roleName)
		} else {
			return err
		}
	} else {
		return err
	}
	return nil
}

func (c *awsClient) DeleteIAMRolePolicy(ctx context.Context, policyName string) error {
	log := log.FromContext(ctx)

	iam_ := iam.New(c.sess)

	if _, err := iam_.DeleteRolePolicy(&iam.DeleteRolePolicyInput{
		PolicyName: &policyName,
		RoleName:   &policyName,
	}); err == nil {
		log.Info("Policy deleted", "policy", policyName)
		return nil
	} else if aerr, ok := err.(awserr.Error); ok {
		if aerr.Code() == iam.ErrCodeNoSuchEntityException {
			// Policy does not exist. Continue.
			log.Info("Policy does not exist", "policy", policyName)
		} else {
			return err
		}
	} else {
		return err
	}
	return nil
}
