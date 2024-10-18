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

	corev1alpha1 "github.com/EO-DataHub/eodhp-workspace-controller/api/v1alpha1"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/route53"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type Route53Reconciler struct {
	client.Client
	AWS AWSClient
}

func (r *Route53Reconciler) Reconcile(ctx context.Context,
	spec *corev1alpha1.WorkspaceSpec,
	status *corev1alpha1.WorkspaceStatus) error {

	log := log.FromContext(ctx)

	// Construct the Route53 URL
	url := fmt.Sprintf("%s.%s", spec.Namespace, r.AWS.config.URL)

	// Create a new Route53 client
	svc := route53.New(r.AWS.sess)

	// Create a Route53 record
	_, err := svc.ChangeResourceRecordSetsWithContext(ctx, &route53.ChangeResourceRecordSetsInput{
		HostedZoneId: aws.String(r.AWS.config.RecordHostedZoneID), // the HostedZoneId of where the new records will be created
		ChangeBatch: &route53.ChangeBatch{
			Changes: []*route53.Change{
				{
					Action: aws.String(route53.ChangeActionUpsert),
					ResourceRecordSet: &route53.ResourceRecordSet{
						Name: aws.String(url),
						Type: aws.String(route53.RRTypeA),
						AliasTarget: &route53.AliasTarget{ // Use AliasTarget instead of ResourceRecords
							DNSName:              aws.String(r.AWS.config.DNSName),
							EvaluateTargetHealth: aws.Bool(true),
							HostedZoneId:         aws.String(r.AWS.config.AliasHostedZoneID), // Hosted zone for alias
						},
					},
				},
			},
		},
	})
	if err != nil {
		log.Error(err, "Failed to create Route53 record")
		return err
	}
	return nil
}

func (r *Route53Reconciler) Teardown(ctx context.Context,
	spec *corev1alpha1.WorkspaceSpec,
	status *corev1alpha1.WorkspaceStatus) error {

	log := log.FromContext(ctx)
	svc := route53.New(r.AWS.sess)

	// Construct the Route53 URL
	url := fmt.Sprintf("%s.%s", spec.Namespace, r.AWS.config.URL)

	// Delete the Route53 record
	_, err := svc.ChangeResourceRecordSetsWithContext(ctx, &route53.ChangeResourceRecordSetsInput{
		HostedZoneId: aws.String(r.AWS.config.RecordHostedZoneID),
		ChangeBatch: &route53.ChangeBatch{
			Changes: []*route53.Change{
				{
					Action: aws.String(route53.ChangeActionDelete),
					ResourceRecordSet: &route53.ResourceRecordSet{
						Name: aws.String(url),             // Replace with your desired record name
						Type: aws.String(route53.RRTypeA), // Replace with your desired record type
						AliasTarget: &route53.AliasTarget{ // Use AliasTarget instead of ResourceRecords
							DNSName:              aws.String(r.AWS.config.DNSName),
							EvaluateTargetHealth: aws.Bool(true),
							HostedZoneId:         aws.String(r.AWS.config.AliasHostedZoneID), // Hosted zone of the load balancer
						},
					},
				},
			},
		},
	})
	if err != nil {
		log.Error(err, "Failed to delete Route53 record")
		return err
	}
	return nil
}
