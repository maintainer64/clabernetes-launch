# CHANGES (branch: cms)

CHANGES (branch: cms) — short, focused on HTTPRoute/Gateway API and ttyd

1. HTTPRoute / Gateway API
    - Added HTTPRoute reconciler + tests (controllers/topology/httproute.go[_test.go]) and registered Gateway API types
      in the controller scheme.
    - Controller ensures expected HTTPRoutes exist for nodes with ttyd enabled and prunes extras.

**Topology/global config (example TTYDHttpRoute):**

```yaml
ttydHttpRoute:
  hostnameSuffix: ".example.com"
  parentRefs:
    - name: gateway-name
      namespace: default
      sectionName: http
```

2. ttyd (web terminal)
    - Launcher starts ttyd for web terminal access (launcher/clabernetes.go); image now includes ttyd (
      build/launcher.Dockerfile) and a simple tmux config (build/launcher/.tmux.conf).
    - API: new TTYDHttpRoute config in apis/v1alpha1; containerlab node fields added: launcher-image, ttyd-shell.
    - Deployment/Service expose ttyd port (7681) and pass TTYD_SHELL into launcher pods.

**Per-node containerlab snippet (node enables ttyd)**

```yaml
nodes:
  node1:
    launcher-image: "ghcr.io/maintainer64/clabernetes-launcher:latest"
    ttyd-shell: "/bin/bash"
```
