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

		openunisonDeployment, err := openunison.NewSateliteDeployment(namespace, operatorImage, operatorDeployCrd, operatorChart, orchestraChart, orchestraLoginPortalChart, pathToValuesYaml, secretFile, controlPlaneCtxName, sateliteCtxName, addClusterChart, pathToSateliteYaml, parseChartSlices(&additionalCharts), parseChartSlices(&preCharts))

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

	installSateliteCmd.PersistentFlags().StringVarP(&operatorImage, "operator-image", "p", "docker.io/tremolosecurity/openunison-k8s-operator:latest", "Operator image name")
	installSateliteCmd.PersistentFlags().BoolVarP(&operatorDeployCrd, "operator-deploy-crds", "d", true, "Deploy CRDs with the operator")
	installSateliteCmd.PersistentFlags().StringVarP(&operatorChart, "operator-chart", "o", "tremolo/openunison-operator", "Helm chart for OpenUnison's operator")

	installSateliteCmd.PersistentFlags().StringVarP(&orchestraChart, "orchestra-chart", "c", "tremolo/orchestra", "Helm chart of the orchestra portal")
	installSateliteCmd.PersistentFlags().StringVarP(&orchestraLoginPortalChart, "orchestra-login-portal-chart", "l", "tremolo/orchestra-login-portal", "Helm chart for the orchestra login portal")
	installSateliteCmd.PersistentFlags().StringVarP(&addClusterChart, "add-cluster-chart", "a", "tremolo/openunison-k8s-add-cluster", "Helm chart fir adding a cluster to OpenUnison")

	installSateliteCmd.PersistentFlags().StringVarP(&pathToSateliteYaml, "save-satelite-values-path", "s", "", "If specified, the values generated for the satelite integration on the control plane are saved to this path")

	preCharts = make([]string, 0)
	additionalCharts = make([]string, 0)

	installAuthPortalCmd.PersistentFlags().StringSliceVarP(&preCharts, "prerun-helm-charts", "u", []string{}, "Comma seperated list of chart=path to deploy charts before OpenUnison is deployed")
	installSateliteCmd.PersistentFlags().StringSliceVarP(&additionalCharts, "additional-helm-charts", "r", []string{}, "Comma seperated list of chart=path to deploy additional charts after OpenUnison is deployed")
}
