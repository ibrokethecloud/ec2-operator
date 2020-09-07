## ec2-operator

Launch and manage ec2 instances using K8S.

The project supports to crds:
* Instance
* ImportKeyPair

### Instance
The Instance type can be used to launch AWS EC2 instances in your account.

Sample manifest is as follows:
```
apiVersion: ec2.cattle.io/v1alpha1
kind: Instance
metadata:
  name: instance-demo
spec:
  # Add fields here
  credentialSecret: aws-secret
  imageID: ami-0051f0f3f07a8934a
  subnetID: subnet-4e1db116
  region: ap-southeast-2
  securityGroupIDS:
    - sg-072a1fd5523cb961a
  publicIPAddress: true
  instanceType: t2.medium
  userData: base64encoded-string-here
```

*Note*: userdata passed to the instance needs to be a base64 encoded string.
 
### ImportKeyPair
The ImportKeyPair type can be used to create a KeyPair in AWS using your custom public key.

Sample manifest is as follows:
```
apiVersion: ec2.cattle.io/v1alpha1
kind: ImportKeyPair
metadata:
  name: importkeypair-sample
spec:
  keyName: mycustom-import-keypair
  publicKey: base64encodedkey
  tagSpecification:
    - name: MyTag
      value: MyValue
  credentialSecret: k8s-secret-with-aws-keypair
  region: aws-region
```
*Note*: publicKey needs to be base64 encoded string.

For both custom types the secret is a k8s secret which contains the keys `aws_access_key` and `aws_secret_key`

Easiest way to generate one is follows:

```
kubectl create secret aws-secret --from-literal=aws_access_key="MYACCESSKEY" --from-literal=aws_secret_key="MYSECRETKEY" -n operator-namespace
```

To get started a helm chart is available [here.](./chart/ec2-operator)

Quick installation:

```
kubectl create namepsace ec2-operator
helm install ec2-operator ./chart/ec2-operator -n ec2-operator
```