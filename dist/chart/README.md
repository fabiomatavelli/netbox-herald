# netbox-herald

![Version: 0.1.0](https://img.shields.io/badge/Version-0.1.0-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: 0.1.0](https://img.shields.io/badge/AppVersion-0.1.0-informational?style=flat-square)

A Helm chart to distribute netbox-herald

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| certManager | object | `{"enabled":false}` | cert-manager integration for TLS certificates; required for webhook certificates and metrics endpoint certificates |
| crd | object | `{"enabled":true,"keep":true}` | Custom Resource Definitions |
| crd.enabled | bool | `true` | Install CRDs with the chart |
| crd.keep | bool | `true` | Keep CRDs when uninstalling the release |
| manager | object | `{"affinity":{},"args":["--leader-elect"],"enabled":true,"env":[{"name":"POD_NAMESPACE","valueFrom":{"fieldRef":{"fieldPath":"metadata.namespace"}}}],"envOverrides":{},"image":{"pullPolicy":"IfNotPresent","repository":"controller"},"nodeSelector":{},"podSecurityContext":{"runAsNonRoot":true,"seccompProfile":{"type":"RuntimeDefault"}},"replicas":1,"resources":{"limits":{"cpu":"500m","memory":"128Mi"},"requests":{"cpu":"10m","memory":"64Mi"}},"securityContext":{"allowPrivilegeEscalation":false,"capabilities":{"drop":["ALL"]},"readOnlyRootFilesystem":true},"terminationGracePeriodSeconds":10,"tolerations":[]}` | Configure the controller manager deployment |
| manager.affinity | object | `{}` | Manager pod's affinity |
| manager.args | list | `["--leader-elect"]` | Manager container arguments |
| manager.enabled | bool | `true` | Set to false to skip manager installation |
| manager.env | list | `[{"name":"POD_NAMESPACE","valueFrom":{"fieldRef":{"fieldPath":"metadata.namespace"}}}]` | Manager container environment variables |
| manager.envOverrides | object | `{}` | Env overrides (--set manager.envOverrides.VAR=value); a matching name in env above takes precedence |
| manager.image.pullPolicy | string | `"IfNotPresent"` | Image pull policy |
| manager.image.repository | string | `"controller"` | Manager image repository |
| manager.nodeSelector | object | `{}` | Manager pod's node selector |
| manager.podSecurityContext | object | `{"runAsNonRoot":true,"seccompProfile":{"type":"RuntimeDefault"}}` | Pod-level security settings |
| manager.replicas | int | `1` | Number of manager replicas |
| manager.resources | object | `{"limits":{"cpu":"500m","memory":"128Mi"},"requests":{"cpu":"10m","memory":"64Mi"}}` | Resource limits and requests |
| manager.securityContext | object | `{"allowPrivilegeEscalation":false,"capabilities":{"drop":["ALL"]},"readOnlyRootFilesystem":true}` | Container-level security settings |
| manager.terminationGracePeriodSeconds | int | `10` | Termination grace period seconds |
| manager.tolerations | list | `[]` | Manager pod's tolerations |
| metrics | object | `{"enabled":true,"port":8443,"secure":true}` | Controller metrics endpoint; enable to expose /metrics |
| metrics.enabled | bool | `true` | Expose the manager's /metrics endpoint |
| metrics.port | int | `8443` | Metrics server port |
| metrics.secure | bool | `true` | Serve metrics over HTTPS with certs/auth (true) or plain HTTP (false); HTTPS requires ClusterRole access for metrics authn/authz |
| networkPolicy | object | `{"enabled":false}` | Network policies for controlling traffic flow; enable to restrict ingress to the controller manager |
| prometheus | object | `{"enabled":false}` | Prometheus ServiceMonitor for metrics scraping; requires prometheus-operator installed in the cluster |
| rbac | object | `{"helpers":{"enabled":false},"namespaced":false}` | RBAC configuration |
| rbac.helpers.enabled | bool | `false` | Install convenience admin/editor/viewer ClusterRoles for the CRDs |
| rbac.namespaced | bool | `false` | RBAC resource scope: false (default) installs a ClusterRole/ClusterRoleBinding covering all namespaces; true installs a namespaced Role/RoleBinding for the release namespace only |
| serviceAccount | object | `{"enabled":true}` | ServiceAccount configuration |
| serviceAccount.enabled | bool | `true` | Install the default ServiceAccount provided by this chart |

