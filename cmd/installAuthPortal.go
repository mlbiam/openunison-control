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

		openunisonDeployment, err := openunison.NewOpenUnisonDeployment(namespace, operatorImage, operatorDeployCrd, operatorChart, orchestraChart, orchestraLoginPortalChart, pathToValuesYaml, secretFile, clusterManagementChart, pathToDbPassword, pathToSmtpPassword)

		if err != nil {
			panic(err)
		}

		if openunisonDeployment.IsNaas() {
			err = openunisonDeployment.DeployNaaSPortal()
			if err != nil {
				panic(err)
			}
		} else {
			err = openunisonDeployment.DeployAuthPortal()
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

	installAuthPortalCmd.PersistentFlags().StringVarP(&operatorImage, "operator-image", "p", "docker.io/tremolosecurity/openunison-k8s-operator:latest", "Operator image name")
	installAuthPortalCmd.PersistentFlags().BoolVarP(&operatorDeployCrd, "operator-deploy-crds", "d", true, "Deploy CRDs with the operator")
	installAuthPortalCmd.PersistentFlags().StringVarP(&operatorChart, "operator-chart", "o", "tremolo/openunison-operator", "Helm chart for OpenUnison's operator")

	installAuthPortalCmd.PersistentFlags().StringVarP(&orchestraChart, "orchestra-chart", "c", "tremolo/orchestra", "Helm chart of the orchestra portal")
	installAuthPortalCmd.PersistentFlags().StringVarP(&orchestraLoginPortalChart, "orchestra-login-portal-chart", "l", "tremolo/orchestra-login-portal", "Helm chart for the orchestra login portal")
	installAuthPortalCmd.PersistentFlags().StringVarP(&secretFile, "secrets-file-path", "s", "", "Path to file containing the authentication secret")

	installAuthPortalCmd.PersistentFlags().StringVarP(&clusterManagementChart, "cluster-management-chart", "m", "tremolo/openunison-k8s-cluster-management", "Helm chart for enabling cluster management")
	installAuthPortalCmd.PersistentFlags().StringVarP(&pathToDbPassword, "database-secret-path", "b", "", "Path to file containing the database password")
	installAuthPortalCmd.PersistentFlags().StringVarP(&pathToSmtpPassword, "smtp-secret-path", "t", "", "Path to file containing the smtp password")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// installAuthPortalCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
