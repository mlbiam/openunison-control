package openunison

import "fmt"

type OperatorDeployment struct {
	image     string
	deployCrd bool
	chart     string
}

// tracks the information about the deployment
type OpenUnisonDeployment struct {
	namespace                 string
	operator                  OperatorDeployment
	orchestraChart            string
	orchestraLoginPortalChart string
	pathToValuesYaml          string
	secretFile                string
}

// creates a new deployment structure
func NewOpenUnisonDeployment(namespace string, operatorImage string, operatorDeployCrd bool, operatorChart string, orchestraChart string, orchestraLoginPortalChart string, pathToValuesYaml string, secretFile string) *OpenUnisonDeployment {
	ou := &OpenUnisonDeployment{}

	ou.namespace = namespace

	ou.operator.image = operatorImage
	ou.operator.deployCrd = operatorDeployCrd
	ou.operator.chart = operatorChart

	ou.orchestraChart = orchestraChart
	ou.orchestraLoginPortalChart = orchestraLoginPortalChart
	ou.pathToValuesYaml = pathToValuesYaml
	ou.secretFile = secretFile

	return ou
}

// deploys OpenUnison into the cluster
func (ou *OpenUnisonDeployment) DeployAuthPortal() {
	fmt.Println("here")
}
