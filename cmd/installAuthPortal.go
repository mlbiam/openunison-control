/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>

*/
package cmd

import (
	"errors"

	"github.com/spf13/cobra"
	"github.com/tremolosecurity/openunison-control/openunison"
)

var operatorImage string
var operatorDeployCrd bool
var operatorChart string

var orchestraChart string
var orchestraLoginPortalChart string
var pathToValuesYaml string
var secretFile string

// installAuthPortalCmd represents the installAuthPortal command
var installAuthPortalCmd = &cobra.Command{
	Use:   "install-auth-portal",
	Short: "Deploys the authentication portal for Kubernetes, requires two arguments: The path to the values.yaml and path to the secrets file",
	Long:  ``,
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) < 2 {
			return errors.New("Requires two arguments: The path to the values.yaml and the path to a secrets file")
		}

		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {

		pathToValuesYaml = args[0]
		secretFile = args[1]

		openunisonDeployment := openunison.NewOpenUnisonDeployment(namespace, operatorImage, operatorDeployCrd, operatorChart, orchestraChart, orchestraLoginPortalChart, pathToValuesYaml, secretFile)

		openunisonDeployment.DeployAuthPortal()

	},
}

func init() {
	rootCmd.AddCommand(installAuthPortalCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:

	installAuthPortalCmd.PersistentFlags().StringVarP(&operatorImage, "operator-image", "p", "docker.io/tremolosecurity/openunison-operator:latest", "Operator image name")
	installAuthPortalCmd.PersistentFlags().BoolVarP(&operatorDeployCrd, "operator-deploy-crds", "d", true, "Deploy CRDs with the operator")
	installAuthPortalCmd.PersistentFlags().StringVarP(&operatorChart, "operator-chart", "o", "tremolo/openunison-operator", "Helm chart for OpenUnison's operator")

	installAuthPortalCmd.PersistentFlags().StringVarP(&orchestraChart, "orchestra-chart", "c", "tremolo/orchestra", "Helm chart of the orchestra portal")
	installAuthPortalCmd.PersistentFlags().StringVarP(&orchestraLoginPortalChart, "orchestra-login-portal-chart", "l", "tremolo/orchestra-login-portal", "Helm chart for the orchestra login portal")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// installAuthPortalCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
