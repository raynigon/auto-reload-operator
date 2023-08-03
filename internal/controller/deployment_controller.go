/*
Copyright 2023 Simon Schneider.

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
	"time"

	"github.com/raynigon/auto-reload-operator/internal/service"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// DeploymentReconciler reconciles a Deployment object
type DeploymentReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=deployments/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=apps,resources=deployments/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Deployment object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *DeploymentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	repo := service.Repository

	var deployment appsv1.Deployment
	exists, err := getResource(ctx, r, req.NamespacedName, &deployment)
	if err != nil {
		return ctrl.Result{}, err
	}
	if !exists {
		err := deleteDeployment(repo, req.NamespacedName)
		return ctrl.Result{}, err
	}

	value, ok := deployment.ObjectMeta.Annotations["auto-reload.raynigon.com/configMap"]
	// If the annotation is not set, we don't care about this deployment
	if !ok {
		return ctrl.Result{}, nil
	}

	// Convert annotation value to NamespacedName
	configMapId, err := stringToNamespacedName(value)
	if err != nil {
		logger.Error(err, "Invalid annotation format", "deployment", req.NamespacedName.String(), "annotation", value)
		return ctrl.Result{}, nil
	}

	// Fetch the config map entity from the repository
	entity, err := repo.FindById(configMapId)
	if err != nil {
		logger.Error(err, "Unable to find config map entity", "deployment", req.NamespacedName.String(), "configMap", configMapId.String())
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	// Check if deployment is already in the list, if so we don't need to do anything
	for _, deployment := range entity.Deployments {
		if deployment == req.NamespacedName {
			return ctrl.Result{}, nil
		}
	}

	// Add the deployment to the config map entity
	entity.Deployments = append(entity.Deployments, req.NamespacedName)

	// Save the modified config map entity
	entity, err = repo.Save(entity)
	if err != nil {
		logger.Error(err, "Unable to save config map entity", "deployment", req.NamespacedName.String(), "configMap", configMapId.String())
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DeploymentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1.Deployment{}).
		Complete(r)
}

func deleteDeployment(repo service.ConfigMapRepository, deploymentId types.NamespacedName) error {
	entities, err := repo.FindByDeployment(deploymentId)
	if err != nil {
		return err
	}
	for _, entity := range entities {
		for i, deployment := range entity.Deployments {
			if deployment == deploymentId {
				entity.Deployments = remove(entity.Deployments, i)
				break
			}
		}
		repo.Save(entity)
	}
	return nil
}

func remove(slice []types.NamespacedName, s int) []types.NamespacedName {
	return append(slice[:s], slice[s+1:]...)
}
