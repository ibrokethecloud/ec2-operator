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

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/ibrokethecloud/ec2-operator/pkg/ec2"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ec2v1alpha1 "github.com/ibrokethecloud/ec2-operator/pkg/api/v1alpha1"
)

// ImportKeyPairReconciler reconciles a ImportKeyPair object
type ImportKeyPairReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=ec2.cattle.io,resources=importkeypairs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ec2.cattle.io,resources=importkeypairs/status,verbs=get;update;patch

func (r *ImportKeyPairReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	keypairFinalizer := "keypair.cattle.io"
	ctx := context.Background()
	log := r.Log.WithValues("importkeypair", req.NamespacedName)

	var keypair ec2v1alpha1.ImportKeyPair
	if err := r.Get(ctx, req.NamespacedName, &keypair); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		log.Error(err, "unable to fetch keypair")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// check if the k8s secret exists before processing item //
	secret, ok, err := r.secretExists(ctx, keypair)
	if !ok {
		log.Error(fmt.Errorf("unable to fetch secret"), keypair.ObjectMeta.Name)
		// Want to requeue as secret may popup later
		return ctrl.Result{RequeueAfter: 30 * time.Second}, err
	}

	// Create new awsClient for this instance to manage it //
	awsClient, err := ec2.NewAWSClient(*secret, keypair.Spec.Region)
	if err != nil {
		log.Info("Error creating AWS Client")
		return ctrl.Result{}, err
	}

	if keypair.ObjectMeta.DeletionTimestamp.IsZero() {
		status := ec2v1alpha1.ImportKeyPairStatus{}
		// only create if keypair.Status.Status is empty
		if keypair.Status.Status == "" {
			status, err = awsClient.ImportKeyPair(keypair)
		} else {
			// Ignore otherwise
			return ctrl.Result{}, nil
		}

		if err != nil {
			log.Info("Error during keypair creation")
			return ctrl.Result{}, err
		}
		controllerutil.AddFinalizer(&keypair, keypairFinalizer)
		keypair.Status = status

		if err := r.Update(ctx, &keypair); err != nil {
			log.Info("Error updating the keypair status and finalizer")
			return ctrl.Result{}, err
		}
	} else {

		if err := awsClient.DeleteKeyPair(keypair); err != nil {
			log.Info("Error deleting keypair")
			return ctrl.Result{}, err
		}
		controllerutil.RemoveFinalizer(&keypair, keypairFinalizer)
		if err := r.Update(ctx, &keypair); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *ImportKeyPairReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ec2v1alpha1.ImportKeyPair{}).
		Complete(r)
}

func (r *ImportKeyPairReconciler) secretExists(ctx context.Context, keypair ec2v1alpha1.ImportKeyPair) (secret *corev1.Secret, ok bool, err error) {
	if len(keypair.Spec.Secret) == 0 {
		return nil, false, fmt.Errorf("No secret specified in InstanceSpec. Will be ignored")
	}
	secret = &corev1.Secret{}
	namespacedSecret := types.NamespacedName{Namespace: keypair.Namespace, Name: keypair.Spec.Secret}
	r.Log.Info("Fetching secret: ", "secret", namespacedSecret)
	err = r.Get(ctx, namespacedSecret, secret)
	if err != nil {
		return nil, false, err
	}

	return secret, true, nil

}
