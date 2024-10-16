/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"errors"

	"github.com/spf13/cobra"
	"github.com/tremolosecurity/openunison-control/openunison"
)

// installSateliteCmd represents the installSatelite command
var installSateliteCmd = &cobra.Command{
	Use:   "install-satelite",
	Short: "Installs a satelite OpenUnison that relies on a control-plane openunison for authentication",
	Long: `This command will deploy an OpenUnison into a satelite cluster for authentication into that cluster using openid connect, using a control-plane OpenUnison as the identity provider.  It will:
	1.  Verify that a cluster with the same name isn't in use
	2.  Create the appropriate Secret in the control plane
	3.  Generate the correct oidc configuration for the satelite and write it to the values file supplied by this command
	4.  Deploy openunison into the satelite cluster
	5.  Deploy the add-cluster chart into the control-plane cluster`,
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) < 3 {
			return errors.New("requires three arguments: The path to the values.yaml, the control plane context name and the satelite context name")
		}

		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		pathToValuesYaml = args[0]
		controlPlaneCtxName := args[1]
		sateliteCtxName := args[2]

		openunisonDeployment, err := openunison.NewSateliteDeployment(namespace, operatorChart, orchestraChart, orchestraLoginPortalChart, pathToValuesYaml, secretFile, controlPlaneCtxName, sateliteCtxName, addClusterChart, pathToSateliteYaml, parseChartSlices(&additionalCharts), parseChartSlices(&preCharts), parseNamespaceLabels(&namespaceLabels), controlPlaneOrchestraChartName, controlPlaneSecretName, skipCPIntegration)

		if err != nil {
			panic(err)
		}

		err = openunisonDeployment.DeployOpenUnisonSatelite()
		if err != nil {
			panic(err)
		}
	},
}

func init() {
	rootCmd.AddCommand(installSateliteCmd)

	installSateliteCmd.PersistentFlags().StringVarP(&operatorChart, "operator-chart", "o", "tremolo/openunison-operator", "Helm chart for OpenUnison's operator, adding '@version' installs the specific version")

	installSateliteCmd.PersistentFlags().StringVarP(&orchestraChart, "orchestra-chart", "c", "tremolo/orchestra", "Helm chart of the orchestra portal, adding '@version' installs the specific version")
	installSateliteCmd.PersistentFlags().StringVarP(&orchestraLoginPortalChart, "orchestra-login-portal-chart", "l", "tremolo/orchestra-login-portal", "Helm chart for the orchestra login portal, adding '@version' installs the specific version")
	installSateliteCmd.PersistentFlags().StringVarP(&addClusterChart, "add-cluster-chart", "a", "tremolo/openunison-k8s-add-cluster", "Helm chart for adding a cluster to OpenUnison, adding '@version' installs the specific version")

	installSateliteCmd.PersistentFlags().StringVarP(&pathToSateliteYaml, "save-satelite-values-path", "s", "", "If specified, the values generated for the satelite integration on the control plane are saved to this path")

	preCharts = make([]string, 0)
	additionalCharts = make([]string, 0)

	installSateliteCmd.PersistentFlags().StringSliceVarP(&preCharts, "prerun-helm-charts", "u", []string{}, "Comma separated list of chart=path to deploy charts before OpenUnison is deployed, adding '@version' installs the specific version")
	installSateliteCmd.PersistentFlags().StringSliceVarP(&additionalCharts, "additional-helm-charts", "r", []string{}, "Comma separated list of chart=path to deploy additional charts after OpenUnison is deployed, adding '@version' installs the specific version")

	installSateliteCmd.PersistentFlags().StringSliceVarP(&namespaceLabels, "namespace-labels", "j", []string{}, "Comma separated list of name=value of labels to add to the openunison namespace")
	installSateliteCmd.PersistentFlags().StringVarP(&controlPlaneOrchestraChartName, "control-plane-orchestra-chart-name", "q", "orchestra", "The name of the orchestra chart on the control plane")
	installSateliteCmd.PersistentFlags().StringVarP(&controlPlaneSecretName, "control-plane-secret-name", "w", "orchestra-secrets-source", "The name of the secret on the control plane to store client secrets in")

	installSateliteCmd.PersistentFlags().BoolVarP(&skipCPIntegration, "skip-controlplane-integration", "k", false, "Set to true if skipping the control plane integration step.  Used when upgrading a satelite.")
}
