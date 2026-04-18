package controllers

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	observabilityv1alpha1 "github.com/sridharkancham/otel-k8s-operator/api/v1alpha1"
)

type OtelCollectorReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=observability.sridharkancham.io,resources=otelcollectors,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=observability.sridharkancham.io,resources=otelcollectors/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps,resources=deployments;daemonsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=configmaps;services,verbs=get;list;watch;create;update;patch;delete

func (r *OtelCollectorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("otelcollector", req.NamespacedName)

	otelCollector := &observabilityv1alpha1.OtelCollector{}
	if err := r.Get(ctx, req.NamespacedName, otelCollector); err != nil {
		if errors.IsNotFound(err) {
			log.Info("OtelCollector resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get OtelCollector")
		return ctrl.Result{}, err
	}

	found := &appsv1.Deployment{}
	err := r.Get(ctx, types.NamespacedName{Name: otelCollector.Name, Namespace: otelCollector.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		dep := r.deploymentForOtelCollector(otelCollector)
		log.Info("Creating a new Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
		if err = r.Create(ctx, dep); err != nil {
			log.Error(err, "Failed to create new Deployment")
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		log.Error(err, "Failed to get Deployment")
		return ctrl.Result{}, err
	}

	replicas := otelCollector.Spec.Replicas
	if *found.Spec.Replicas != replicas {
		found.Spec.Replicas = &replicas
		if err = r.Update(ctx, found); err != nil {
			log.Error(err, "Failed to update Deployment")
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	otelCollector.Status.AvailableReplicas = found.Status.AvailableReplicas
	if err := r.Status().Update(ctx, otelCollector); err != nil {
		log.Error(err, "Failed to update OtelCollector status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *OtelCollectorReconciler) deploymentForOtelCollector(m *observabilityv1alpha1.OtelCollector) *appsv1.Deployment {
	ls := labelsForOtelCollector(m.Name)
	replicas := m.Spec.Replicas

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      m.Name,
			Namespace: m.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: ls},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: ls},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Image: fmt.Sprintf("otel/opentelemetry-collector-contrib:%s", m.Spec.Version),
						Name:  "otel-collector",
						Ports: []corev1.ContainerPort{
							{ContainerPort: 4317, Name: "otlp-grpc"},
							{ContainerPort: 4318, Name: "otlp-http"},
							{ContainerPort: 8889, Name: "prometheus"},
						},
						Resources: m.Spec.Resources,
					}},
				},
			},
		},
	}

	ctrl.SetControllerReference(m, dep, r.Scheme)
	return dep
}

func labelsForOtelCollector(name string) map[string]string {
	return map[string]string{
		"app":                          "otel-collector",
		"otelcollector_cr":             name,
		"app.kubernetes.io/managed-by": "otel-k8s-operator",
	}
}

func (r *OtelCollectorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&observabilityv1alpha1.OtelCollector{}).
		Owns(&appsv1.Deployment{}).
		Complete(r)
}
