package deployment

import (
	"context"

	routev1 "github.com/openshift/api/route/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// CreateNaysayerRoute creates the OpenShift route programmatically
func CreateNaysayerRoute(namespace, domain string) *routev1.Route {
	return &routev1.Route{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "route.openshift.io/v1",
			Kind:       "Route",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "naysayer",
			Namespace: namespace,
			Labels: map[string]string{
				"app":       "naysayer",
				"component": "webhook",
			},
		},
		Spec: routev1.RouteSpec{
			Host: "naysayer-webhook." + domain,
			To: routev1.RouteTargetReference{
				Kind:   "Service",
				Name:   "naysayer",
				Weight: &[]int32{100}[0],
			},
			Port: &routev1.RoutePort{
				TargetPort: intstr.FromString("http"),
			},
			TLS: &routev1.TLSConfig{
				Termination:                   routev1.TLSTerminationEdge,
				InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
			},
			WildcardPolicy: routev1.WildcardPolicyNone,
		},
	}
}

// DeployRoute applies the route to the cluster
func DeployRoute(ctx context.Context, client routev1.RouteInterface, namespace, domain string) error {
	route := CreateNaysayerRoute(namespace, domain)
	_, err := client.Create(ctx, route, metav1.CreateOptions{})
	return err
}