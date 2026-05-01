package topology

import (
	"fmt"
	"sort"

	clabernetesapisv1alpha1 "github.com/srl-labs/clabernetes/apis/v1alpha1"
	clabernetesconfig "github.com/srl-labs/clabernetes/config"
	clabernetesconstants "github.com/srl-labs/clabernetes/constants"
	claberneteserrors "github.com/srl-labs/clabernetes/errors"
	claberneteslogging "github.com/srl-labs/clabernetes/logging"
	clabernetesutil "github.com/srl-labs/clabernetes/util"
	clabernetesutilcontainerlab "github.com/srl-labs/clabernetes/util/containerlab"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apimachinerytypes "k8s.io/apimachinery/pkg/types"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// HTTPRouteReconciler is a subcomponent of the "TopologyReconciler" but is exposed for testing
// purposes. This is the component responsible for rendering/validating HTTPRoutes for a
// clabernetes topology resource.
type HTTPRouteReconciler struct {
	log                 claberneteslogging.Instance
	configManagerGetter clabernetesconfig.ManagerGetterFunc
}

// NewHTTPRouteReconciler returns an instance of HTTPRouteReconciler.
func NewHTTPRouteReconciler(
	log claberneteslogging.Instance,
	configManagerGetter clabernetesconfig.ManagerGetterFunc,
) *HTTPRouteReconciler {
	return &HTTPRouteReconciler{
		log:                 log,
		configManagerGetter: configManagerGetter,
	}
}

// Resolve accepts a list of HTTPRoutes that are associated with the topology and returns an
// ObjectDiffer containing missing, extra, and current HTTPRoutes.
func (r *HTTPRouteReconciler) Resolve(
	ownedHTTPRoutes *gatewayv1.HTTPRouteList,
	clabernetesConfigs map[string]*clabernetesutilcontainerlab.Config,
	_ *clabernetesapisv1alpha1.Topology,
) (*clabernetesutil.ObjectDiffer[*gatewayv1.HTTPRoute], error) {
	httpRoutes := &clabernetesutil.ObjectDiffer[*gatewayv1.HTTPRoute]{
		Current: map[string]*gatewayv1.HTTPRoute{},
	}

	for i := range ownedHTTPRoutes.Items {
		labels := ownedHTTPRoutes.Items[i].Labels

		if labels == nil {
			return nil, fmt.Errorf(
				"%w: labels are nil, but we expect to see topology owner label here",
				claberneteserrors.ErrInvalidData,
			)
		}

		nodeName, ok := labels[clabernetesconstants.LabelTopologyNode]
		if !ok || nodeName == "" {
			return nil, fmt.Errorf(
				"%w: topology node label is missing or empty",
				claberneteserrors.ErrInvalidData,
			)
		}

		httpRoutes.Current[nodeName] = &ownedHTTPRoutes.Items[i]
	}

	// Build set of node names that should have HTTPRoutes (nodes with ttyd-shell enabled)
	ttydIngress := r.configManagerGetter().GetTTYDHttpRoute()
	expectedNodes := make([]string, 0)

	if ttydIngress != nil && ttydIngress.HostnameSuffix != "" {
		for nodeName, nodeTopo := range clabernetesConfigs {
			if nodeTopo.Topology == nil || nodeTopo.Topology.Nodes == nil {
				continue
			}

			nodeDef, ok := nodeTopo.Topology.Nodes[nodeName]
			if !ok || nodeDef == nil {
				continue
			}

			if nodeDef.TTYDShell != "" {
				expectedNodes = append(expectedNodes, nodeName)
			}
		}
	}

	httpRoutes.SetMissing(expectedNodes)
	httpRoutes.SetExtra(expectedNodes)

	return httpRoutes, nil
}

// Render renders HTTPRoutes for a given node. Returns both redirect and backend routes.
func (r *HTTPRouteReconciler) Render(
	owningTopology *clabernetesapisv1alpha1.Topology,
	nodeName string,
) []*gatewayv1.HTTPRoute {
	ttydIngress := r.configManagerGetter().GetTTYDHttpRoute()
	if ttydIngress == nil || ttydIngress.HostnameSuffix == "" {
		return nil
	}

	owningTopologyName := owningTopology.GetName()
	hostname := fmt.Sprintf(
		"%s-%s-%s%s",
		owningTopologyName,
		nodeName,
		owningTopology.Namespace,
		ttydIngress.HostnameSuffix,
	)
	httpRouteAnnotationName := fmt.Sprintf("%s-%s", owningTopologyName, nodeName)
	serviceName := fmt.Sprintf("%s-%s-vx", owningTopologyName, nodeName)

	if ResolveTopologyRemovePrefix(owningTopology) {
		hostname = fmt.Sprintf(
			"%s-%s%s",
			nodeName,
			owningTopology.Namespace,
			ttydIngress.HostnameSuffix,
		)
		httpRouteAnnotationName = nodeName
		serviceName = fmt.Sprintf("%s-vx", nodeName)
	}

	// Convert parent refs
	parentRefs := make([]gatewayv1.ParentReference, len(ttydIngress.ParentRefs))
	for i, ref := range ttydIngress.ParentRefs {
		parentRefs[i] = gatewayv1.ParentReference{
			Name:        gatewayv1.ObjectName(ref.Name),
			Namespace:   (*gatewayv1.Namespace)(&ref.Namespace),
			SectionName: (*gatewayv1.SectionName)(&ref.SectionName),
		}
	}

	routes := make([]*gatewayv1.HTTPRoute, 0, 2) //nolint:mnd

	// Redirect route (http -> https)
	redirectRoute := &gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-redirect", httpRouteAnnotationName),
			Namespace: owningTopology.Namespace,
			Labels: map[string]string{
				clabernetesconstants.LabelTopologyOwner: owningTopology.GetName(),
				clabernetesconstants.LabelTopologyNode:  nodeName,
			},
		},
		Spec: gatewayv1.HTTPRouteSpec{
			CommonRouteSpec: gatewayv1.CommonRouteSpec{
				ParentRefs: parentRefs,
			},
			Hostnames: []gatewayv1.Hostname{
				gatewayv1.Hostname(hostname),
			},
			Rules: []gatewayv1.HTTPRouteRule{
				{
					Filters: []gatewayv1.HTTPRouteFilter{
						{
							Type: gatewayv1.HTTPRouteFilterRequestRedirect,
							RequestRedirect: &gatewayv1.HTTPRequestRedirectFilter{
								Scheme:     clabernetesutil.ToPointer("https"),
								StatusCode: clabernetesutil.ToPointer(301), //nolint:mnd
							},
						},
					},
				},
			},
		},
	}

	routes = append(routes, redirectRoute)

	// HTTPS backend route
	httpsRefs := make([]gatewayv1.ParentReference, len(parentRefs))
	for i, ref := range parentRefs {
		sectionName := gatewayv1.SectionName("https")
		httpsRefs[i] = gatewayv1.ParentReference{
			Name:        ref.Name,
			Namespace:   ref.Namespace,
			SectionName: &sectionName,
		}
	}

	backendRoute := &gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      httpRouteAnnotationName,
			Namespace: owningTopology.Namespace,
			Labels: map[string]string{
				clabernetesconstants.LabelTopologyOwner: owningTopology.GetName(),
				clabernetesconstants.LabelTopologyNode:  nodeName,
			},
		},
		Spec: gatewayv1.HTTPRouteSpec{
			CommonRouteSpec: gatewayv1.CommonRouteSpec{
				ParentRefs: httpsRefs,
			},
			Hostnames: []gatewayv1.Hostname{
				gatewayv1.Hostname(hostname),
			},
			Rules: []gatewayv1.HTTPRouteRule{
				{
					BackendRefs: []gatewayv1.HTTPBackendRef{
						{
							BackendRef: gatewayv1.BackendRef{
								BackendObjectReference: gatewayv1.BackendObjectReference{
									Name: gatewayv1.ObjectName(serviceName),
									Port: clabernetesutil.ToPointer(
										gatewayv1.PortNumber(clabernetesconstants.TTYDServicePort),
									),
								},
							},
						},
					},
				},
			},
		},
	}

	routes = append(routes, backendRoute)

	return routes
}

// RenderAll renders HTTPRoutes for all given node names.
func (r *HTTPRouteReconciler) RenderAll(
	owningTopology *clabernetesapisv1alpha1.Topology,
	nodeNames []string,
) []*gatewayv1.HTTPRoute {
	var allRoutes []*gatewayv1.HTTPRoute

	for _, nodeName := range nodeNames {
		routes := r.Render(owningTopology, nodeName)
		allRoutes = append(allRoutes, routes...)
	}

	return allRoutes
}

// Conforms checks if the existing HTTPRoute conforms with the rendered HTTPRoute.
func (r *HTTPRouteReconciler) Conforms(
	existing *gatewayv1.HTTPRoute,
	rendered *gatewayv1.HTTPRoute,
	expectedOwnerUID apimachinerytypes.UID,
) bool {
	if len(existing.Spec.ParentRefs) != len(rendered.Spec.ParentRefs) {
		return false
	}

	for i := range rendered.Spec.ParentRefs {
		if existing.Spec.ParentRefs[i].Name != rendered.Spec.ParentRefs[i].Name {
			return false
		}
	}

	if len(existing.Spec.Hostnames) != len(rendered.Spec.Hostnames) {
		return false
	}

	// Sort hostnames for comparison
	existingHostnames := make([]string, len(existing.Spec.Hostnames))
	for i, h := range existing.Spec.Hostnames {
		existingHostnames[i] = string(h)
	}

	renderedHostnames := make([]string, len(rendered.Spec.Hostnames))
	for i, h := range rendered.Spec.Hostnames {
		renderedHostnames[i] = string(h)
	}

	sort.Strings(existingHostnames)
	sort.Strings(renderedHostnames)

	for i := range renderedHostnames {
		if existingHostnames[i] != renderedHostnames[i] {
			return false
		}
	}

	if len(existing.Spec.Rules) != len(rendered.Spec.Rules) {
		return false
	}

	if len(existing.OwnerReferences) != 1 {
		return false
	}

	if existing.OwnerReferences[0].UID != expectedOwnerUID {
		return false
	}

	return true
}
