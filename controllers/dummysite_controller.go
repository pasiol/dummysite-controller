/*
Copyright 2021.

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
	"crypto/rand"
	"encoding/hex"
	"fmt"
	kbatch "k8s.io/api/batch/v1"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"math"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	stabledwkv1 "stable.dwk/api/v1"
)

// DummySiteReconciler reconciles a DummySite object
type DummySiteReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

var (
	jobOwnerKey = ".metadata.controller"
	apiGVStr    = stabledwkv1.GroupVersion.String()
)

func randomBase16String(l int) string {
	buff := make([]byte, int(math.Round(float64(l)/2)))
	rand.Read(buff)
	str := hex.EncodeToString(buff)
	return str[:l] // strip 1 extra character we get from odd length results
}

func constructJobForDummySite(dummysite *stabledwkv1.DummySite, r *DummySiteReconciler) (*kbatch.Job, error) {
	// We want job names for a given nominal start time to have a deterministic name to avoid the same job being created twice
	name := fmt.Sprintf(dummysite.Name)
	//dep := &v1.Deployment{}

	job := &kbatch.Job{
		ObjectMeta: metav1.ObjectMeta{
			Labels:      make(map[string]string),
			Annotations: make(map[string]string),
			Name:        name + "-job" + "-" + randomBase16String(6),
			Namespace:   dummysite.Namespace,
		},
		Spec: kbatch.JobSpec{
			Template: core.PodTemplateSpec{
				Spec: core.PodSpec{
					Containers: []core.Container{
						{
							Name:  name + "-job-test",
							Image: dummysite.Spec.Image,
						},
					},
					RestartPolicy: "Never",
				},
			},
		},
	}

	if err := ctrl.SetControllerReference(dummysite, job, r.Scheme); err != nil {
		return nil, err
	}

	return job, nil
}

//+kubebuilder:rbac:groups=stable.dwk.stable.dwk,resources=dummysites,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=stable.dwk.stable.dwk,resources=dummysites/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=stable.dwk.stable.dwk,resources=dummysites/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the DummySite object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.10.0/pkg/reconcile
func (r *DummySiteReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)
	var dummySite stabledwkv1.DummySite
	if err := r.Get(ctx, req.NamespacedName, &dummySite); err != nil {
		fmt.Println(err, "unable to fetch DummySite")
	}

	job, err := constructJobForDummySite(&dummySite, r)

	if err := r.Create(ctx, job); err != nil {
		fmt.Println(err, "unable to create Job for ", "job")
		return ctrl.Result{}, err
	}
	if err != nil {
		fmt.Println(err, "unable to construct job from template")
		return ctrl.Result{}, err
	}
	fmt.Println("created Job for DummySite run", job.Name)
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DummySiteReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &kbatch.Job{}, jobOwnerKey, func(rawObj client.Object) []string {
		// grab the job object, extract the owner...
		job := rawObj.(*kbatch.Job)
		owner := metav1.GetControllerOf(job)
		if owner == nil {
			return nil
		}
		// ...make sure it's a Countdown...

		if owner.APIVersion != apiGVStr || owner.Kind != "Countdown" {
			return nil
		}

		// ...and if so, return it
		return []string{owner.Name}
	}); err != nil {

		return err
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&stabledwkv1.DummySite{}).
		Complete(r)
}
