/*


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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// InstanceSpec defines the desired state of Instance
type InstanceSpec struct {
	BlockDeviceMapping string   `json:"blockDeviceMapping,omitempty"`
	ImageID            string   `json:"imageID"`
	InstanceType       string   `json:"instanceType"`
	KeyName            string   `json:"keyname,omitempty"`
	SecurityGroupIDS   []string `json:"securityGroupIDS,omitempty"`
	SecurityGroups     []string `json:"securityGroups,omitempty"`
	SubnetID           string   `json:"subnetID,omitempty"`
	UserData           string   `json:"userData,omitempty"`
	IAMInstanceProfile string   `json:"iamInstanceProfile,omitempty"`
	TagSpecifications  []Tags   `json:"tagSpecification,omitempty"`
	Secret             string   `json:"credentialSecret"` // K8S secret containing the account creds //
	PublicIPAddress    bool     `json:"publicIPAddress,omitEmpty"`
	Region             string   `json:"region"`
}

type Tags struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// InstanceStatus defines the observed state of Instance
type InstanceStatus struct {
	Status     string `json:"status"`
	InstanceID string `json:"instanceID"`
	PrivateIP  string `json:"privateIP"`
	PublicIP   string `json:"publicIP"`
}

// +kubebuilder:object:root=true

// Instance is the Schema for the instances API
type Instance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   InstanceSpec   `json:"spec,omitempty"`
	Status InstanceStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// InstanceList contains a list of Instance
type InstanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Instance `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Instance{}, &InstanceList{})
}
