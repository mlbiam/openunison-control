package openunison

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"time"

	"gopkg.in/yaml.v3"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
)

var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890")

func randSeq(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

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
	secret                    string
	clientset                 *kubernetes.Clientset
}

// creates a new deployment structure
func NewOpenUnisonDeployment(namespace string, operatorImage string, operatorDeployCrd bool, operatorChart string, orchestraChart string, orchestraLoginPortalChart string, pathToValuesYaml string, secretFile string) (*OpenUnisonDeployment, error) {
	ou := &OpenUnisonDeployment{}

	ou.namespace = namespace

	ou.operator.image = operatorImage
	ou.operator.deployCrd = operatorDeployCrd
	ou.operator.chart = operatorChart

	ou.orchestraChart = orchestraChart
	ou.orchestraLoginPortalChart = orchestraLoginPortalChart
	ou.pathToValuesYaml = pathToValuesYaml
	ou.secretFile = secretFile

	err := ou.loadKubernetesConfiguration()
	if err != nil {
		return nil, err
	}

	return ou, nil
}

// get the current k8s configuration

func (ou *OpenUnisonDeployment) loadKubernetesConfiguration() error {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	configOverrides := &clientcmd.ConfigOverrides{}
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)

	config, err := kubeConfig.ClientConfig()
	if err != nil {
		return err
	}

	ou.clientset, err = kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}

	return nil
}

// deploys OpenUnison into the cluster
func (ou *OpenUnisonDeployment) DeployAuthPortal() error {
	fmt.Printf("Loading values from %s...\n", ou.pathToValuesYaml)

	yamlValues, err := ioutil.ReadFile(ou.pathToValuesYaml)

	if err != nil {
		return err
	}

	helmValues := make(map[string]interface{})

	err = yaml.Unmarshal(yamlValues, &helmValues)

	if err != nil {
		return err
	}

	fmt.Printf("...loaded\n")

	// check the kubernetes dashboard ns exists

	dashboardNamespace := "kubernetes-dashboard"

	dashboardConfig, ok := helmValues["dashboard"].(map[interface{}]interface{})
	if ok {

		dashboardNamespace = dashboardConfig["namespace"].(string)
	}

	ou.checkNamespace("Dashboard", dashboardNamespace)

	// check the openunison namespace exists, if not, create it

	ou.checkNamespace("OpenUnison", ou.namespace)

	// create the orchestra-secrets-source

	secret, err := ou.clientset.CoreV1().Secrets(ou.namespace).Get(context.TODO(), "orchestra-secrets-source", metav1.GetOptions{})

	if err != nil {
		// secret doesn't exist, need to create it
		secret = &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "orchestra-secrets-source",
				Namespace: ou.namespace,
			},
			Data: map[string][]byte{},
		}

		// generate the standard keys
		secret.Data["unisonKeystorePassword"] = []byte(randSeq((64)))
		secret.Data["K8S_DB_SECRET"] = []byte(randSeq((64)))

		hasSecret := ou.secretFile != "" || ou.secret == ""

		var authSecret []byte

		if hasSecret {
			if ou.secret != "" {
				authSecret = []byte(ou.secret)
			} else {
				authSecret, err = ioutil.ReadFile(ou.secretFile)
				if err != nil {
					return err
				}
			}

		}

		authNeedsSecret := false
		authSecretLabel := ""
		foundAuth := false

		_, ok = helmValues["oidc"]

		if ok {
			authNeedsSecret = true
			foundAuth = true
			authSecretLabel = "OIDC_CLIENT_ID"
		} else {
			_, ok = helmValues["github"]

			if ok {
				foundAuth = true
				authNeedsSecret = true
				authSecretLabel = "GITHUB_SECRET_ID"
			} else {
				_, ok = helmValues["active_directory"]

				if ok {
					authNeedsSecret = true
					foundAuth = true
					authSecretLabel = "AD_BIND_PASSWORD"
				} else {
					_, ok = helmValues["saml"]
					authNeedsSecret = false
					foundAuth = true

				}
			}
		}

		if !foundAuth {
			return fmt.Errorf("No authentication found, one of active_directory, github, oidc, saml required")
		}

		if authNeedsSecret && !hasSecret {
			return fmt.Errorf("Authentication type requires a secret in a file specified in -s or --secrets-file-path")
		}

		secret.Data[authSecretLabel] = authSecret

		secret, err = ou.clientset.CoreV1().Secrets(ou.namespace).Create(context.TODO(), secret, metav1.CreateOptions{})

		if err != nil {
			return err
		} else {
			fmt.Println("Secret created")
		}
	} else {
		fmt.Print("Secret already exists\n")
	}

	// deploy the operator
	settings := cli.New()
	actionConfig := new(action.Configuration)

	if err := actionConfig.Init(settings.RESTClientGetter(), ou.namespace, os.Getenv("HELM_DRIVER"), log.Printf); err != nil {
		return err
	}

	listClient := action.NewList(actionConfig)

	openunisonDeployed := false
	orchestraDeployed := false
	orchestraLoginPortalDeployed := false

	listClient.All = true
	releases, err := listClient.Run()

	for _, release := range releases {
		if release.Name == "openunison" && release.Namespace == ou.namespace {
			openunisonDeployed = true
		}

		if release.Name == "orchestra" && release.Namespace == ou.namespace {
			orchestraDeployed = true
		}

		if release.Name == "orchestra-login-portal" && release.Namespace == ou.namespace {
			orchestraLoginPortalDeployed = true
		}
	}

	//if !openunisonDeployed {
	fmt.Print("Deploying the OpenUnison Operator\n")

	if !openunisonDeployed {
		fmt.Println("Chart not deployed, installing")
		client := action.NewInstall(actionConfig)

		client.Namespace = ou.namespace
		client.ReleaseName = "openunison"

		cp, err := client.ChartPathOptions.LocateChart(ou.operator.chart, settings)

		if err != nil {
			return err
		}

		chartReq, err := loader.Load(cp)

		if err != nil {
			return err
		}

		// define values
		vals := map[string]interface{}{
			"image":      ou.operator.image,
			"crd.deploy": ou.operator.deployCrd,
		}

		_, err = client.Run(chartReq, vals)

		if err != nil {
			return err
		}
	} else {
		fmt.Println("Chart deployed, upgrading")
		client := action.NewUpgrade(actionConfig)

		client.Namespace = ou.namespace

		cp, err := client.ChartPathOptions.LocateChart(ou.operator.chart, settings)

		if err != nil {
			return err
		}

		chartReq, err := loader.Load(cp)

		if err != nil {
			return err
		}

		// define values
		vals := map[string]interface{}{
			"image":      ou.operator.image,
			"crd.deploy": ou.operator.deployCrd,
		}

		_, err = client.Run("openunison", chartReq, vals)

		if err != nil {
			return err
		}
	}

	// wait until the operator is up and running

	err = waitForDeployment(ou, "openunison-operator")
	if err != nil {
		return err
	}

	fmt.Println("Checking for a previously failed run")

	_, err = ou.clientset.CoreV1().Pods(ou.namespace).Get(context.TODO(), "test-orchestra-orchestra", metav1.GetOptions{})

	if err == nil {
		fmt.Println("test-orchestra-orchestra Pod exists, deleting")

		err = ou.clientset.CoreV1().Pods(ou.namespace).Delete(context.TODO(), "test-orchestra-orchestra", metav1.DeleteOptions{})

		if err != nil {
			return err
		}

		fmt.Println("Deleted test-orchestra-orchestra")
	}

	fmt.Println("Deploying the orchestra chart")

	var deployErr error

	if !orchestraDeployed {
		// deploy orchestra, make sure that it deploys

		fmt.Println("Orchestra doesn't exist, installing")

		client := action.NewInstall(actionConfig)

		client.Namespace = ou.namespace
		client.ReleaseName = "orchestra"

		cp, err := client.ChartPathOptions.LocateChart(ou.orchestraChart, settings)

		if err != nil {
			return err
		}

		chartReq, err := loader.Load(cp)

		if err != nil {
			return err
		}

		mergedValues := mergeMaps(chartReq.Values, helmValues)

		_, deployErr = client.Run(chartReq, mergedValues)

	} else {
		// deploy orchestra, make sure that it deploys

		fmt.Println("Orchestra exists, upgrading")

		client := action.NewUpgrade(actionConfig)

		client.Namespace = ou.namespace

		cp, err := client.ChartPathOptions.LocateChart(ou.orchestraChart, settings)

		if err != nil {
			return err
		}

		chartReq, err := loader.Load(cp)

		if err != nil {
			return err
		}

		mergedValues := mergeMaps(chartReq.Values, helmValues)

		_, deployErr = client.Run("orchestra", chartReq, mergedValues)
	}

	if deployErr != nil {
		_, err = ou.clientset.CoreV1().Pods(ou.namespace).Get(context.TODO(), "test-orchestra-orchestra", metav1.GetOptions{})

		if err == nil {
			fmt.Println("Failed prechecks:")

			req := ou.clientset.CoreV1().Pods(ou.namespace).GetLogs("test-orchestra-orchestra", &v1.PodLogOptions{})
			podLogs, err := req.Stream(context.TODO())
			if err != nil {
				return err
			}
			defer podLogs.Close()

			buf := new(bytes.Buffer)
			_, err = io.Copy(buf, podLogs)
			if err != nil {
				return err
			}
			str := buf.String()

			fmt.Println(str)
			return nil
		}

		return err
	}

	// wait until the orchestra container is running

	err = waitForDeployment(ou, "openunison-orchestra")
	if err != nil {
		return err
	}

	fmt.Println("Deploying the orchestra-login-portal chart")

	if !orchestraLoginPortalDeployed {

		// deploy the orchestra-login-portal charts
		fmt.Println("orchestra-login-portal not deployed, installing")

		client := action.NewInstall(actionConfig)

		client.Namespace = ou.namespace
		client.ReleaseName = "orchestra-login-portal"

		cp, err := client.ChartPathOptions.LocateChart(ou.orchestraLoginPortalChart, settings)

		if err != nil {
			return err
		}

		chartReq, err := loader.Load(cp)

		if err != nil {
			return err
		}

		mergedValues := mergeMaps(chartReq.Values, helmValues)

		_, err = client.Run(chartReq, mergedValues)

		if err != nil {
			return err
		}

		// wait until the orchestra container is running

		err = waitForDeployment(ou, "ouhtml-orchestra-login-portal")
		if err != nil {
			return err
		}

		network := mergedValues["network"].(map[string]interface{})
		ouHost := network["openunison_host"]

		fmt.Printf("OpenUnison is deployed!  Visit https://%v/ to login to your cluster!\n", ouHost)
	} else {
		// deploy the orchestra-login-portal charts
		fmt.Println("orchestra-login-portal deployed, upgrading")

		client := action.NewUpgrade(actionConfig)

		client.Namespace = ou.namespace

		cp, err := client.ChartPathOptions.LocateChart(ou.orchestraLoginPortalChart, settings)

		if err != nil {
			return err
		}

		chartReq, err := loader.Load(cp)

		if err != nil {
			return err
		}

		mergedValues := mergeMaps(chartReq.Values, helmValues)

		_, err = client.Run("orchestra-login-portal", chartReq, mergedValues)

		if err != nil {
			return err
		}

		// wait until the orchestra container is running

		err = waitForDeployment(ou, "ouhtml-orchestra-login-portal")
		if err != nil {
			return err
		}

		network := mergedValues["network"].(map[string]interface{})
		ouHost := network["openunison_host"]

		fmt.Printf("OpenUnison is deployed!  Visit https://%v/ to login to your cluster!\n", ouHost)
	}
	// all done!

	return nil
}

func waitForDeployment(ou *OpenUnisonDeployment, deploymentName string) error {
	running := false

	for i := 0; i < 200; i++ {
		fmt.Printf("Checking for the %s %v\n", deploymentName, i)

		dep, err := ou.clientset.AppsV1().Deployments(ou.namespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})

		if err != nil {
			return err
		}

		fmt.Printf("Ready : %v\n", dep.Status.ReadyReplicas)

		if dep.Status.ReadyReplicas > 0 {
			running = true
			break
		}

		time.Sleep(1 * time.Second)
	}

	if !running {
		return fmt.Errorf("Timed out waiting for the openunison operator chart to be deployed")
	} else {
		fmt.Printf("Deployment %v is Running\n", deploymentName)
	}
	return nil
}

func (ou *OpenUnisonDeployment) checkNamespace(label string, name string) error {

	fmt.Printf("Checking for the %s namespace %s\n", label, name)

	_, err := ou.clientset.CoreV1().Namespaces().Get(context.TODO(), name, metav1.GetOptions{})

	if err != nil {
		fmt.Printf("%s namespace %s does not exist, creating\n", label, name)
		_, err = ou.clientset.CoreV1().Namespaces().Create(context.TODO(), &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}}, metav1.CreateOptions{})
		if err != nil {
			return err
		}
		fmt.Printf("%s namespace %s created\n", label, name)
	} else {
		fmt.Printf("%s namespace %s already exists\n", label, name)
	}

	return nil
}

func mergeMaps(base, overlay map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(base))
	for key, value := range base {
		out[key] = value
	}

	for k, v := range overlay {
		if v, ok := v.(map[string]interface{}); ok {
			if bv, ok := out[k]; ok {
				if bv, ok := bv.(map[string]interface{}); ok {
					out[k] = mergeMaps(bv, v)
					continue
				}
			}
		}
		out[k] = v
	}
	return out
}
