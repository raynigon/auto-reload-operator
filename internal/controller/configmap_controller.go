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
	"bytes"
	"context"
	"crypto/sha512"
	"encoding/base64"
	"encoding/gob"
	"time"

	"github.com/raynigon/auto-reload-operator/internal/service"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// ConfigMapReconciler reconciles a ConfigMap object
type ConfigMapReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=configmaps/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=core,resources=configmaps/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ConfigMap object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *ConfigMapReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	repo := service.Repository

	var configMap corev1.ConfigMap
	exists, err := getResource(ctx, r, req.NamespacedName, &configMap)
	if err != nil {
		logger.Error(err, "Unable to fetch ConfigMap", "configMap", req.NamespacedName.String())
		return ctrl.Result{}, err
	}

	// Handle deleted
	if !exists {
		err := repo.Delete(req.NamespacedName)
		if err != nil {
			logger.Error(err, "Unable to delete ConfigMap", "configMap", req.NamespacedName.String())
			return ctrl.Result{}, err
		}
		logger.Info("Deleted ConfigMap", "database", req.NamespacedName.String())
		return ctrl.Result{}, nil
	}

	// Handle created
	entity, err := repo.FindById(req.NamespacedName)
	if err != nil {
		entity = service.ConfigMapEntity{
			Id:       req.NamespacedName,
			Pods:     []types.NamespacedName{},
			DataHash: "",
		}
		entity, err = repo.Save(entity)
		if err != nil {
			logger.Error(err, "Unable to save ConfigMapEntity", "configMap", req.NamespacedName.String())
			return ctrl.Result{}, err
		}
	}

	currentDataHash := hashData(configMap)
	// Exit if the hash did not change
	if entity.DataHash == currentDataHash {
		return ctrl.Result{}, nil
	}

	// Restart all pods that reference this config map
	for _, pod := range entity.Pods {
		err := restartPod(ctx, r.Client, pod)
		if err != nil {
			logger.Error(err, "Unable to restart Pod", "pod", pod.String())
			return ctrl.Result{}, err
		}
	}

	// Update the data hash in the entity and save it
	entity.DataHash = currentDataHash
	entity, err = repo.Save(entity)
	if err != nil {
		logger.Error(err, "Unable to save ConfigMapEntity", "configMap", req.NamespacedName.String())
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ConfigMapReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.ConfigMap{}).
		Complete(r)
}

func hashData(cm corev1.ConfigMap) string {
	var b bytes.Buffer
	// Encode the data and binary data of the config map into a byte array
	gob.NewEncoder(&b).Encode(cm.Data)
	gob.NewEncoder(&b).Encode(cm.BinaryData)
	// Generate the hash from the byte array
	hash := sha512.New().Sum(b.Bytes())
	// Convert the hash to a base64 string
	return base64.StdEncoding.EncodeToString(hash)
}

func restartPod(ctx context.Context, r client.Client, id types.NamespacedName) error {
	logger := log.FromContext(ctx)
	var pod corev1.Pod
	exists, err := getResource(ctx, r, id, &pod)
	if err != nil {
		logger.Error(err, "Unable to fetch Pod", "pod", id.String())
		return err
	}
	if !exists {
		logger.Info("Pod does not exist", "pod", pod.String())
		return nil
	}
	// Restart the pod by updating the restart annotation
	pod.ObjectMeta.Annotations["auto-reload.raynigon.com/restartedAt"] = time.Now().Format(time.RFC3339)
	return r.Update(ctx, &pod)
}
