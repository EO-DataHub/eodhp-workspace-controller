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
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
)

type AWSConfig struct {
	AccountID string `yaml:"accountID"`
	Region    string `yaml:"region"`
	OIDC      struct {
		Provider string `yaml:"provider"`
	}
	URL string `yaml:"url"`
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
	} else {
		fmt.Println("Error creating AWS session ", err)
		return err
	}

	return nil
}

func (c *AWSClient) Enabled() bool {
	return c.sess != nil
}
