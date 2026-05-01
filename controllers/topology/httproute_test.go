package topology_test

import (
	"testing"

	clabernetesapisv1alpha1 "github.com/srl-labs/clabernetes/apis/v1alpha1"
	clabernetesconfig "github.com/srl-labs/clabernetes/config"
	clabernetesconstants "github.com/srl-labs/clabernetes/constants"
	clabernetescontrollerstopology "github.com/srl-labs/clabernetes/controllers/topology"
	claberneteslogging "github.com/srl-labs/clabernetes/logging"
	clabernetesutil "github.com/srl-labs/clabernetes/util"
	clabernetesutilcontainerlab "github.com/srl-labs/clabernetes/util/containerlab"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

const renderHTTPRouteTestName = "httproute/render-httproute"

func TestResolveHTTPRoute(t *testing.T) {
	cases := []struct {
		name               string
		ownedHTTPRoutes    *gatewayv1.HTTPRouteList
		ttydIngress        *clabernetesapisv1alpha1.TTYDHttpRoute
		clabernetesConfigs map[string]*clabernetesutilcontainerlab.Config
		expectedCurrent    []string
		expectedMissing    []string
		expectedExtra      []*gatewayv1.HTTPRoute
	}{
		{
			name:            "no-ttyd-ingress",
			ownedHTTPRoutes: &gatewayv1.HTTPRouteList{},
			ttydIngress:     nil,
			clabernetesConfigs: map[string]*clabernetesutilcontainerlab.Config{
				"node1": {
					Topology: &clabernetesutilcontainerlab.Topology{
						Nodes: map[string]*clabernetesutilcontainerlab.NodeDefinition{
							"node1": {
								TTYDShell: "/bin/bash",
							},
						},
					},
				},
			},
			expectedCurrent: nil,
			expectedMissing: nil,
			expectedExtra:   []*gatewayv1.HTTPRoute{},
		},
		{
			name:            "simple",
			ownedHTTPRoutes: &gatewayv1.HTTPRouteList{},
			ttydIngress: &clabernetesapisv1alpha1.TTYDHttpRoute{
				HostnameSuffix: ".example.com",
				ParentRefs: []clabernetesapisv1alpha1.TTYDHttpRouteParentRef{
					{Name: "gw", Namespace: "default", SectionName: "http"},
				},
			},
			clabernetesConfigs: map[string]*clabernetesutilcontainerlab.Config{
				"node1": {
					Topology: &clabernetesutilcontainerlab.Topology{
						Nodes: map[string]*clabernetesutilcontainerlab.NodeDefinition{
							"node1": {
								TTYDShell: "/bin/bash",
							},
						},
					},
				},
			},
			expectedCurrent: nil,
			expectedMissing: []string{"node1"},
			expectedExtra:   []*gatewayv1.HTTPRoute{},
		},
		{
			name: "with-existing-routes",
			ownedHTTPRoutes: &gatewayv1.HTTPRouteList{
				Items: []gatewayv1.HTTPRoute{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "topo-node1-redirect",
							Namespace: "default",
							Labels: map[string]string{
								clabernetesconstants.LabelTopologyNode: "node1",
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "topo-node1",
							Namespace: "default",
							Labels: map[string]string{
								clabernetesconstants.LabelTopologyNode: "node1",
							},
						},
					},
				},
			},
			ttydIngress: &clabernetesapisv1alpha1.TTYDHttpRoute{
				HostnameSuffix: ".example.com",
				ParentRefs: []clabernetesapisv1alpha1.TTYDHttpRouteParentRef{
					{Name: "gw", Namespace: "default", SectionName: "http"},
				},
			},
			clabernetesConfigs: map[string]*clabernetesutilcontainerlab.Config{
				"node1": {
					Topology: &clabernetesutilcontainerlab.Topology{
						Nodes: map[string]*clabernetesutilcontainerlab.NodeDefinition{
							"node1": {
								TTYDShell: "/bin/bash",
							},
						},
					},
				},
			},
			expectedCurrent: []string{"node1"},
			expectedMissing: nil,
			expectedExtra:   []*gatewayv1.HTTPRoute{},
		},
		{
			name: "extra-routes",
			ownedHTTPRoutes: &gatewayv1.HTTPRouteList{
				Items: []gatewayv1.HTTPRoute{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "topo-node2",
							Namespace: "default",
							Labels: map[string]string{
								clabernetesconstants.LabelTopologyNode: "node2",
							},
						},
					},
				},
			},
			ttydIngress: &clabernetesapisv1alpha1.TTYDHttpRoute{
				HostnameSuffix: ".example.com",
				ParentRefs: []clabernetesapisv1alpha1.TTYDHttpRouteParentRef{
					{Name: "gw", Namespace: "default", SectionName: "http"},
				},
			},
			clabernetesConfigs: map[string]*clabernetesutilcontainerlab.Config{
				"node1": {
					Topology: &clabernetesutilcontainerlab.Topology{
						Nodes: map[string]*clabernetesutilcontainerlab.NodeDefinition{
							"node1": {
								TTYDShell: "/bin/bash",
							},
						},
					},
				},
			},
			expectedCurrent: nil,
			expectedMissing: []string{"node1"},
			expectedExtra: []*gatewayv1.HTTPRoute{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "topo-node2",
						Namespace: "default",
						Labels: map[string]string{
							clabernetesconstants.LabelTopologyNode: "node2",
						},
					},
				},
			},
		},
	}

	for _, testCase := range cases {
		t.Run(
			testCase.name,
			func(t *testing.T) {
				t.Logf("%s: starting", testCase.name)

				reconciler := clabernetescontrollerstopology.NewHTTPRouteReconciler(
					&claberneteslogging.FakeInstance{},
					func() clabernetesconfig.Manager {
						return &fakeManagerForHTTPRoute{ttydIngress: testCase.ttydIngress}
					},
				)

				got, err := reconciler.Resolve(
					testCase.ownedHTTPRoutes,
					testCase.clabernetesConfigs,
					nil,
				)
				if err != nil {
					t.Fatal(err)
				}

				var gotCurrent []string
				for current := range got.Current {
					gotCurrent = append(gotCurrent, current)
				}

				if !clabernetesutil.StringSliceContainsAll(gotCurrent, testCase.expectedCurrent) {
					t.Errorf("Current: got %v, want %v", gotCurrent, testCase.expectedCurrent)
				}

				if !clabernetesutil.StringSliceContainsAll(got.Missing, testCase.expectedMissing) {
					t.Errorf("Missing: got %v, want %v", got.Missing, testCase.expectedMissing)
				}
			})
	}
}

type fakeManagerForHTTPRoute struct {
	clabernetesconfig.Manager

	ttydIngress *clabernetesapisv1alpha1.TTYDHttpRoute
}

func (f *fakeManagerForHTTPRoute) GetTTYDHttpRoute() *clabernetesapisv1alpha1.TTYDHttpRoute {
	return f.ttydIngress
}

func TestRenderHTTPRoute(t *testing.T) {
	cases := []struct {
		name               string
		owningTopology     *clabernetesapisv1alpha1.Topology
		ttydIngress        *clabernetesapisv1alpha1.TTYDHttpRoute
		clabernetesConfigs map[string]*clabernetesutilcontainerlab.Config
		nodeName           string
		expectedCount      int
	}{
		{
			name: "simple",
			owningTopology: &clabernetesapisv1alpha1.Topology{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "topo",
					Namespace: "default",
				},
			},
			ttydIngress: &clabernetesapisv1alpha1.TTYDHttpRoute{
				HostnameSuffix: ".example.com",
				ParentRefs: []clabernetesapisv1alpha1.TTYDHttpRouteParentRef{
					{Name: "gw", Namespace: "default", SectionName: "http"},
				},
			},
			clabernetesConfigs: map[string]*clabernetesutilcontainerlab.Config{
				"node1": {
					Topology: &clabernetesutilcontainerlab.Topology{
						Nodes: map[string]*clabernetesutilcontainerlab.NodeDefinition{
							"node1": {
								TTYDShell: "/bin/bash",
							},
						},
					},
				},
			},
			nodeName:      "node1",
			expectedCount: 2, // redirect + backend routes
		},
		{
			name: "no-ttyd-ingress",
			owningTopology: &clabernetesapisv1alpha1.Topology{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "topo",
					Namespace: "default",
				},
			},
			ttydIngress: nil,
			clabernetesConfigs: map[string]*clabernetesutilcontainerlab.Config{
				"node1": {
					Topology: &clabernetesutilcontainerlab.Topology{
						Nodes: map[string]*clabernetesutilcontainerlab.NodeDefinition{
							"node1": {
								TTYDShell: "/bin/bash",
							},
						},
					},
				},
			},
			nodeName:      "node1",
			expectedCount: 0,
		},
	}

	for _, testCase := range cases {
		t.Run(
			testCase.name,
			func(t *testing.T) {
				t.Logf("%s: starting", testCase.name)

				reconciler := clabernetescontrollerstopology.NewHTTPRouteReconciler(
					&claberneteslogging.FakeInstance{},
					func() clabernetesconfig.Manager {
						return &fakeManagerForHTTPRoute{ttydIngress: testCase.ttydIngress}
					},
				)

				got := reconciler.Render(
					testCase.owningTopology,
					testCase.nodeName,
				)

				if len(got) != testCase.expectedCount {
					t.Errorf("expected %d routes, got %d", testCase.expectedCount, len(got))
				}
			})
	}
}

func TestConformsHTTPRoute(t *testing.T) {
	t.Run("conforms", func(t *testing.T) {
		reconciler := clabernetescontrollerstopology.NewHTTPRouteReconciler(
			&claberneteslogging.FakeInstance{},
			clabernetesconfig.GetFakeManager,
		)

		existing := &gatewayv1.HTTPRoute{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-route",
				Labels: map[string]string{
					clabernetesconstants.LabelTopologyOwner: "topo",
				},
				OwnerReferences: []metav1.OwnerReference{
					{UID: "test-uid"},
				},
			},
			Spec: gatewayv1.HTTPRouteSpec{
				CommonRouteSpec: gatewayv1.CommonRouteSpec{
					ParentRefs: []gatewayv1.ParentReference{
						{Name: "gw"},
					},
				},
				Hostnames: []gatewayv1.Hostname{"pc-node1.example.com"},
			},
		}

		rendered := &gatewayv1.HTTPRoute{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-route",
			},
			Spec: gatewayv1.HTTPRouteSpec{
				CommonRouteSpec: gatewayv1.CommonRouteSpec{
					ParentRefs: []gatewayv1.ParentReference{
						{Name: "gw"},
					},
				},
				Hostnames: []gatewayv1.Hostname{"pc-node1.example.com"},
			},
		}

		if !reconciler.Conforms(existing, rendered, "test-uid") {
			t.Errorf("expected routes to conform")
		}
	})

	t.Run("does-not-conform", func(t *testing.T) {
		reconciler := clabernetescontrollerstopology.NewHTTPRouteReconciler(
			&claberneteslogging.FakeInstance{},
			clabernetesconfig.GetFakeManager,
		)

		existing := &gatewayv1.HTTPRoute{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-route",
				OwnerReferences: []metav1.OwnerReference{
					{UID: "test-uid"},
				},
			},
			Spec: gatewayv1.HTTPRouteSpec{
				CommonRouteSpec: gatewayv1.CommonRouteSpec{
					ParentRefs: []gatewayv1.ParentReference{
						{Name: "old-gw"},
					},
				},
			},
		}

		rendered := &gatewayv1.HTTPRoute{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-route",
			},
			Spec: gatewayv1.HTTPRouteSpec{
				CommonRouteSpec: gatewayv1.CommonRouteSpec{
					ParentRefs: []gatewayv1.ParentReference{
						{Name: "new-gw"},
					},
				},
			},
		}

		if reconciler.Conforms(existing, rendered, "test-uid") {
			t.Errorf("expected routes to not conform")
		}
	})
}
