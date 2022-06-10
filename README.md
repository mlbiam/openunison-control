# ouctl

This utility automates the deployment of OpenUnison's helm charts into your cluster.  It has helm built in, so it doesn't need to use an external helm binary.  It has two commands, one for deploying a stand-alone OpenUnison instance and one for deploying a satelite instance.  Prior to using this tool, refer to the [OpenUnison deployment guide](https://openunison.github.io/deployauth/) for instructions on how to configure OpenUnison's values.yaml.

## install-auth-portal

This command will deploy a stand-alone OpenUnison instance.  It can deploy as both an [authentication portal](https://openunison.github.io/) and as a [Namespace as a Service (NaaS) portal](https://openunison.github.io/namespace_as_a_service/).  Prior to running this command, a values.yaml file will need to be created.  It is the only required argument for this command.  Optional flags:

```
  -m, --cluster-management-chart string       Helm chart for enabling cluster management (default "tremolo/openunison-k8s-cluster-management")
  -b, --database-secret-path string           Path to file containing the database password
  -h, --help                                  help for install-auth-portal
  -o, --operator-chart string                 Helm chart for OpenUnison's operator (default "tremolo/openunison-operator")
  -d, --operator-deploy-crds                  Deploy CRDs with the operator (default true)
  -p, --operator-image string                 Operator image name (default "docker.io/tremolosecurity/openunison-k8s-operator:latest")
  -c, --orchestra-chart string                Helm chart of the orchestra portal (default "tremolo/orchestra")
  -l, --orchestra-login-portal-chart string   Helm chart for the orchestra login portal (default "tremolo/orchestra-login-portal")
  -s, --secrets-file-path string              Path to file containing the authentication secret
  -t, --smtp-secret-path string               Path to file containing the smtp password`
```

If run on an existing cluster, this command will upgrade existing charts.  For authentication soltuions that require a secret, this command can be re-run without that secret safely.  

## install-satelite

To support [Multi cluster SSO](https://openunison.github.io/multi_cluster_sso/) This command installs a satelite instance of OpenUnison onto a remote instance.  It has three arguments:

1. The path to the new OpenUnison's values.yaml
2. The name of the context in your kubectl configuration file for the control plane Kubernetes cluster
3. The name of the context in your kubectl configuration file for the new satelite cluster

This command will make several changes to your values.yaml to automate the installation, such as configuring the `oidc` section for you.  There's no need to create a secret for this mode, the command will create it for you.

Optional flags:

```
 -a, --add-cluster-chart string              Helm chart fir adding a cluster to OpenUnison (default "tremolo/openunison-k8s-add-cluster")
  -h, --help                                  help for install-satelite
  -o, --operator-chart string                 Helm chart for OpenUnison's operator (default "tremolo/openunison-operator")
  -d, --operator-deploy-crds                  Deploy CRDs with the operator (default true)
  -p, --operator-image string                 Operator image name (default "docker.io/tremolosecurity/openunison-k8s-operator:latest")
  -c, --orchestra-chart string                Helm chart of the orchestra portal (default "tremolo/orchestra")
  -l, --orchestra-login-portal-chart string   Helm chart for the orchestra login portal (default "tremolo/orchestra-login-portal")
  -s, --save-satelite-values-path string      If specified, the values generated for the satelite integration on the control plane are saved to this path
```

This command can be re-run safely.  If charts have already been deployed, they'll be updated.
