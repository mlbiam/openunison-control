/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"errors"

	"github.com/spf13/cobra"
	"github.com/tremolosecurity/openunison-control/openunison"
)

var secretFile string

// installAuthPortalCmd represents the installAuthPortal command
var installAuthPortalCmd = &cobra.Command{
	Use:   "install-auth-portal",
	Short: "Deploys the authentication portal for Kubernetes, requires one argument: The path to the values.yaml",
	Long:  ``,
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) < 1 {
			return errors.New("Requires one argument: The path to the values.yaml")
		}

		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {

		pathToValuesYaml = args[0]

		openunisonDeployment, err := openunison.NewOpenUnisonDeployment(namespace, operatorChart, orchestraChart, orchestraLoginPortalChart, pathToValuesYaml, secretFile, clusterManagementChart, pathToDbPassword, pathToSmtpPassword, skipClusterManagement, parseChartSlices(&additionalCharts), parseChartSlices(&preCharts), parseNamespaceLabels(&namespaceLabels))

		if err != nil {
			panic(err)
		}

		if openunisonDeployment.IsNaas() {
			err = openunisonDeployment.DeployNaaSPortal()
			if err != nil {
				panic(err)
			}

			err = openunisonDeployment.DeployAdditionalCharts()
			if err != nil {
				panic(err)
			}

		} else {
			err = openunisonDeployment.DeployAuthPortal()
			if err != nil {
				panic(err)
			}

			err = openunisonDeployment.DeployAdditionalCharts()
			if err != nil {
				panic(err)
			}
		}

	},
}

func init() {
	rootCmd.AddCommand(installAuthPortalCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:

	installAuthPortalCmd.PersistentFlags().StringVarP(&operatorChart, "operator-chart", "o", "tremolo/openunison-operator", "Helm chart for OpenUnison's operator, adding '@version' installs the specific version")

	installAuthPortalCmd.PersistentFlags().StringVarP(&orchestraChart, "orchestra-chart", "c", "tremolo/orchestra", "Helm chart of the orchestra portal, adding '@version' installs the specific version")
	installAuthPortalCmd.PersistentFlags().StringVarP(&orchestraLoginPortalChart, "orchestra-login-portal-chart", "l", "tremolo/orchestra-login-portal", "Helm chart for the orchestra login portal, adding '@version' installs the specific version")
	installAuthPortalCmd.PersistentFlags().StringVarP(&secretFile, "secrets-file-path", "s", "", "Path to file containing the authentication secret")

	installAuthPortalCmd.PersistentFlags().StringVarP(&clusterManagementChart, "cluster-management-chart", "m", "tremolo/openunison-k8s-cluster-management", "Helm chart for enabling cluster management, adding '@version' installs the specific version")
	installAuthPortalCmd.PersistentFlags().StringVarP(&pathToDbPassword, "database-secret-path", "b", "", "Path to file containing the database password")
	installAuthPortalCmd.PersistentFlags().StringVarP(&pathToSmtpPassword, "smtp-secret-path", "t", "", "Path to file containing the smtp password")

	installAuthPortalCmd.PersistentFlags().BoolVarP(&skipClusterManagement, "skip-cluster-management", "k", false, "Set to true if skipping the cluster management chart when openunison.enable_provisioning is true")

	preCharts = make([]string, 0)
	additionalCharts = make([]string, 0)

	installAuthPortalCmd.PersistentFlags().StringSliceVarP(&preCharts, "prerun-helm-charts", "u", []string{}, "Comma separated list of chart=path to deploy charts before OpenUnison is deployed, adding '@version' installs the specific version")
	installAuthPortalCmd.PersistentFlags().StringSliceVarP(&additionalCharts, "additional-helm-charts", "r", []string{}, "Comma separated list of chart=path to deploy additional charts after OpenUnison is deployed, adding '@version' installs the specific version")

	installAuthPortalCmd.PersistentFlags().StringSliceVarP(&namespaceLabels, "namespace-labels", "j", []string{}, "Comma separated list of name=value of labels to add to the openunison namespace")
	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// installAuthPortalCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
