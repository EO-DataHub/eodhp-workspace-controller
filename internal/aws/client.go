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
	} `yaml:"oidc"`
	URL                string `yaml:"url"`
	RecordHostedZoneID string `yaml:"recordHostedZoneID"`
	DNSName            string `yaml:"dnsName"`
	AliasHostedZoneID  string `yaml:"aliasHostedZoneID"`
	RoleName           string `yaml:"roleName"`
	Prefix             string `yaml:"prefix"`
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
