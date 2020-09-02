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

package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/ibrokethecloud/ec2-operator/pkg/ec2"

	corev1 "k8s.io/api/core/v1"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ec2v1alpha1 "github.com/ibrokethecloud/ec2-operator/pkg/api/v1alpha1"
)

// InstanceReconciler reconciles a Instance object
type InstanceReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=ec2.cattle.io,resources=instances,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ec2.cattle.io,resources=instances/status,verbs=get;update;patch

func (r *InstanceReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	instanceFinalizer := "instance.cattle.io"
	ctx := context.Background()
	log := r.Log.WithValues("instance", req.NamespacedName)

	var instance ec2v1alpha1.Instance

	if err := r.Get(ctx, req.NamespacedName, &instance); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		log.Error(err, "unable to fetch instance")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// check if the k8s secret exists before processing item //
	secret, ok, err := r.secretExists(ctx, instance)
	if !ok {
		log.Error(fmt.Errorf("unable to fetch secret"), instance.ObjectMeta.Name)
		// Want to requeue as secret may popup later
		return ctrl.Result{RequeueAfter: 30 * time.Second}, err
	}

	// Create new awsClient for this instance to manage it //
	awsClient, err := ec2.NewAWSClient(*secret, instance.Spec.Region)
	if err != nil {
		log.Info("Error creating AWS Client")
		return ctrl.Result{}, err
	}
	// Launch a new instance //
	if instance.ObjectMeta.DeletionTimestamp.IsZero() {
		// Check if instance needs to be launched //
		instanceStatus := ec2v1alpha1.InstanceStatus{}
		switch status := instance.Status.Status; status {
		case "":
			log.Info("Creating instance")
			instanceStatus, err = awsClient.CreateInstance(instance)
		case ec2.WaitForPublicIP:
			log.Info("Fetching Public IP")
			instanceStatus, err = awsClient.FetchPublicIP(instance)
		case ec2.WaitForTag:
			log.Info("Updating Tags")
			instanceStatus, err = awsClient.UpdateTags(instance)
		default:
			return ctrl.Result{}, nil
		}

		if err != nil {
			log.Error(fmt.Errorf("Error during instance creation"), instance.ObjectMeta.Name)
			return ctrl.Result{}, err
		}

		instance.Status = instanceStatus
		if !containsString(instance.ObjectMeta.Finalizers, instanceFinalizer) {
			instance.ObjectMeta.Finalizers = append(instance.ObjectMeta.Finalizers, instanceFinalizer)
		}
		if err = r.Update(ctx, &instance); err != nil {
			log.Error(fmt.Errorf("Error while updating status and application of finalizer"), instance.ObjectMeta.Name)
			// Not going to requeue since the instance has already been provisioned //
			return ctrl.Result{}, err
		}
	} else {
		if containsString(instance.ObjectMeta.Finalizers, instanceFinalizer) {
			// lets delete the instance //
			log.Info("Terminating")
			if err = awsClient.DeleteInstance(instance); err != nil {
				log.Error(fmt.Errorf("Error during instance deletion so requeueing"), instance.ObjectMeta.Name)
				return ctrl.Result{}, err
			}
		}

		instance.ObjectMeta.Finalizers = removeString(instance.ObjectMeta.Finalizers, instanceFinalizer)
		if err := r.Update(ctx, &instance); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Requeue object if its not yet completed provisioning
	// Default flow of object is
	// 1.Create Instance
	// 2.Create Tags
	// 3.Check For public IP if specified

	if instance.Status.Status != "provisioned" {
		return ctrl.Result{Requeue: true}, nil
	}

	return ctrl.Result{}, nil
}

func (r *InstanceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ec2v1alpha1.Instance{}).
		Complete(r)
}

// containsString is a helper to check if finalizer exists
func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

// removeString is a helper to remove the finalizer from the object
func removeString(slice []string, s string) (result []string) {
	for _, item := range slice {
		if item == s {
			continue
		}
		result = append(result, item)
	}
	return
}

func (r *InstanceReconciler) secretExists(ctx context.Context, instance ec2v1alpha1.Instance) (secret *corev1.Secret, ok bool, err error) {
	if len(instance.Spec.Secret) == 0 {
		return nil, false, fmt.Errorf("No secret specified in InstanceSpec. Will be ignored")
	}
	secret = &corev1.Secret{}
	namespacedSecret := types.NamespacedName{Namespace: instance.Namespace, Name: instance.Spec.Secret}
	r.Log.Info("Fetching secret: ", "secret", namespacedSecret)
	err = r.Get(ctx, namespacedSecret, secret)
	if err != nil {
		return nil, false, err
	}

	return secret, true, nil

}
