package ec2

import (
	"fmt"

	"github.com/aws/aws-sdk-go/service/ec2"

	"github.com/aws/aws-sdk-go/aws"

	"github.com/aws/aws-sdk-go/aws/session"

	"github.com/aws/aws-sdk-go/aws/credentials"
	awsec2 "github.com/aws/aws-sdk-go/service/ec2"
	ec2v1alpha1 "github.com/hobbyfarm/ec2-operator/pkg/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

var (
	Provisioned     = "provisioned"
	WaitForPublicIP = "waitforpublicip"
	WaitForTag      = "waitfortag"
)

type AWSClient struct {
	svc *awsec2.EC2
}

func NewAWSClient(secret corev1.Secret, region string) (a *AWSClient, err error) {
	creds, err := createCredentials(secret)
	if err != nil {
		return nil, err
	}

	sess, err := session.NewSession(&aws.Config{
		Credentials: creds,
		Region:      aws.String(region),
	})

	svc := ec2.New(sess)

	a = &AWSClient{
		svc: svc,
	}

	return a, nil
}

// CreateInstance will take the instance spec and delete the instance //
func (a *AWSClient) CreateInstance(instance ec2v1alpha1.Instance) (status ec2v1alpha1.InstanceStatus, err error) {
	// For instances that are edited.. we are not going to do ignore //
	var reservation *awsec2.Reservation
	if instance.Status.Status == "Provisioned" && len(instance.Status.InstanceID) > 0 {
		return instance.Status, nil
	}

	if instance.Status.Status == "" {
		runInput := &awsec2.RunInstancesInput{
			ImageId:      aws.String(instance.Spec.ImageID),
			InstanceType: aws.String(instance.Spec.InstanceType),
			MinCount:     aws.Int64(1),
			MaxCount:     aws.Int64(1),
			SubnetId:     aws.String(instance.Spec.SubnetID),
			IamInstanceProfile: &awsec2.IamInstanceProfileSpecification{
				Arn: aws.String(instance.Spec.IAMInstanceProfile),
			},
			UserData:         aws.String(instance.Spec.UserData),
			SecurityGroupIds: aws.StringSlice(instance.Spec.SecurityGroupIDS),
		}
		if len(instance.Spec.KeyName) > 0 {
			runInput = runInput.SetKeyName(instance.Spec.KeyName)
		}
		reservation, err = a.svc.RunInstances(runInput)
	}

	if err != nil {
		return status, err
	}

	status.InstanceID = *reservation.Instances[0].InstanceId
	status.PrivateIP = *reservation.Instances[0].PrivateIpAddress
	status.Status = WaitForTag
	return status, nil
}

// DeleteInstance will remove the instance and
func (a *AWSClient) DeleteInstance(instance ec2v1alpha1.Instance) (err error) {

	_, err = a.svc.TerminateInstances(&awsec2.TerminateInstancesInput{
		InstanceIds: aws.StringSlice([]string{instance.Status.InstanceID}),
	})

	return err
}

func createCredentials(secret corev1.Secret) (creds *credentials.Credentials, err error) {
	access_key, ok := secret.Data["aws_access_key"]
	if !ok {
		return nil, fmt.Errorf("No key aws_access_key exists in Instance secret")
	}

	secret_key, ok := secret.Data["aws_secret_key"]
	if !ok {
		return nil, fmt.Errorf("No key aws_secret_key exists in Instance secret")
	}
	creds = credentials.NewStaticCredentials(string(access_key), string(secret_key), "")
	return creds, nil
}

func (a *AWSClient) FetchPublicIP(instance ec2v1alpha1.Instance) (status ec2v1alpha1.InstanceStatus, err error) {
	describeInstanceOuput, err := a.svc.DescribeInstances(&awsec2.DescribeInstancesInput{
		InstanceIds: aws.StringSlice([]string{instance.Status.InstanceID}),
	})
	if err != nil {
		return status, err
	}

	status = *instance.Status.DeepCopy()
	if describeInstanceOuput.Reservations[0].Instances[0].PublicIpAddress != nil {
		status.PublicIP = *describeInstanceOuput.Reservations[0].Instances[0].PublicIpAddress
		status.Status = Provisioned
		return status, nil
	}

	// Return original status as IP not yet available
	// Will cause requeue of object by the main reconcile loop
	return status, nil
}

func (a *AWSClient) UpdateTags(instance ec2v1alpha1.Instance) (status ec2v1alpha1.InstanceStatus, err error) {
	// tag instance //
	tags := []*awsec2.Tag{}

	for _, tagDetails := range instance.Spec.TagSpecifications {
		tags = append(tags, &awsec2.Tag{Key: aws.String(tagDetails.Name), Value: aws.String(tagDetails.Value)})
	}
	//Default tag
	tags = append(tags, &awsec2.Tag{Key: aws.String("Name"), Value: aws.String(instance.ObjectMeta.Name)})

	_, err = a.svc.CreateTags(&awsec2.CreateTagsInput{
		Resources: []*string{aws.String(instance.Status.InstanceID)},
		Tags:      tags,
	})

	if err != nil {
		return status, err
	}

	status = *instance.Status.DeepCopy()
	if instance.Spec.PublicIPAddress {
		status.Status = WaitForPublicIP
	} else {
		status.Status = Provisioned
	}
	return status, nil
}

func (a *AWSClient) ImportKeyPair(keypair ec2v1alpha1.ImportKeyPair) (status ec2v1alpha1.ImportKeyPairStatus, err error) {
	if len(keypair.Spec.PublicKey) == 0 {
		return status, fmt.Errorf("Empty KeyPair specified")
	}

	// tag instance //
	tags := []*awsec2.Tag{}

	for _, tagDetails := range keypair.Spec.TagSpecifications {
		tags = append(tags, &awsec2.Tag{Key: aws.String(tagDetails.Name), Value: aws.String(tagDetails.Value)})
	}
	//Default tag
	tags = append(tags, &awsec2.Tag{Key: aws.String("Name"), Value: aws.String(keypair.Name)})

	output, err := a.svc.ImportKeyPair(&ec2.ImportKeyPairInput{
		KeyName:           aws.String(keypair.Name),
		PublicKeyMaterial: []byte(keypair.Spec.PublicKey),
		TagSpecifications: []*awsec2.TagSpecification{&awsec2.TagSpecification{
			ResourceType: aws.String("key-pair"),
			Tags:         tags,
		},
		},
	})

	if err != nil {
		status.Status = "error"
		return status, err
	}

	status.Status = "provisioned"
	status.KeyPairID = *output.KeyPairId

	return status, nil
}

func (a *AWSClient) DeleteKeyPair(keypair ec2v1alpha1.ImportKeyPair) (err error) {
	_, err = a.svc.DeleteKeyPair(&ec2.DeleteKeyPairInput{
		KeyPairId: aws.String(keypair.Status.KeyPairID),
	})

	return err
}
