package openunison

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
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

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
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

	controlPlaneContextName string
	satelateContextName     string
	addClusterChart         string
}

// creates a new deployment structure
func NewOpenUnisonDeployment(namespace string, operatorImage string, operatorDeployCrd bool, operatorChart string, orchestraChart string, orchestraLoginPortalChart string, pathToValuesYaml string, secretFile string) (*OpenUnisonDeployment, error) {
	return NewSateliteDeployment(namespace, operatorImage, operatorDeployCrd, operatorChart, orchestraChart, orchestraLoginPortalChart, pathToValuesYaml, secretFile, "", "", "")
}

// creates a new deployment structure
func NewSateliteDeployment(namespace string, operatorImage string, operatorDeployCrd bool, operatorChart string, orchestraChart string, orchestraLoginPortalChart string, pathToValuesYaml string, secretFile string, controlPlanContextName string, sateliteContextName string, addClusterChart string) (*OpenUnisonDeployment, error) {
	ou := &OpenUnisonDeployment{}

	ou.namespace = namespace

	ou.operator.image = operatorImage
	ou.operator.deployCrd = operatorDeployCrd
	ou.operator.chart = operatorChart

	ou.orchestraChart = orchestraChart
	ou.orchestraLoginPortalChart = orchestraLoginPortalChart
	ou.pathToValuesYaml = pathToValuesYaml
	ou.secretFile = secretFile

	ou.controlPlaneContextName = controlPlanContextName
	ou.satelateContextName = sateliteContextName
	ou.addClusterChart = addClusterChart

	err := ou.loadKubernetesConfiguration()
	if err != nil {
		return nil, err
	}

	return ou, nil
}

// set the current k8s context
func (ou *OpenUnisonDeployment) setCurrentContext(ctxName string) (string, error) {
	flag.Parse()

	currentContextName := ctxName

	pathOptions := clientcmd.NewDefaultPathOptions()
	curCfg, err := pathOptions.GetStartingConfig()

	if err != nil {
		return "", err
	}

	if curCfg.CurrentContext != ctxName {

		_, ok := curCfg.Contexts[ctxName]

		if !ok {
			return "", fmt.Errorf("context %s does not exist", ctxName)
		}

		currentContextName = curCfg.CurrentContext
		curCfg.CurrentContext = ctxName

		clientConfig := clientcmd.NewDefaultClientConfig(*curCfg, nil)
		clientcmd.ModifyConfig(clientConfig.ConfigAccess(), *curCfg, false)
	}

	return currentContextName, nil
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

// deploys an OpenUnison satelite
func (ou *OpenUnisonDeployment) DeployOpenUnisonSatelite() error {

	originalContextName, err := ou.setCurrentContext(ou.controlPlaneContextName)

	if err != nil {
		return err
	}

	ou.loadKubernetesConfiguration()

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

	// get the satelite cluster name

	clusterName, ok := helmValues["k8s_cluster_name"].(string)

	if !ok {
		return fmt.Errorf("k8s_cluster_name must be defined in the satalite values.yaml")
	}

	satelateReleaseName := "satelite-" + clusterName

	settings := cli.New()
	actionConfig := new(action.Configuration)

	if err := actionConfig.Init(settings.RESTClientGetter(), ou.namespace, os.Getenv("HELM_DRIVER"), log.Printf); err != nil {
		return err
	}

	listClient := action.NewList(actionConfig)

	listClient.All = true
	releases, err := listClient.Run()

	sateliteIntegrated := false

	for _, release := range releases {
		if release.Namespace == ou.namespace && release.Name == satelateReleaseName {
			sateliteIntegrated = true
		}
	}

	ouSecret, err := ou.clientset.CoreV1().Secrets(ou.namespace).Get(context.TODO(), "orchestra-secrets-source", metav1.GetOptions{})

	if err != nil {
		return err
	}

	sateliteClientSecret, ok := ouSecret.Data["cluster-idp-"+clusterName]

	if !ok {
		fmt.Println("SSO Client Secret doesn't exist, creating")
		ou.secret = string(randSeq((64)))
		ouSecret.Data["cluster-idp-"+clusterName] = []byte(ou.secret)
		ou.clientset.CoreV1().Secrets(ou.namespace).Update(context.TODO(), ouSecret, metav1.UpdateOptions{})
		fmt.Println("Created")
	} else {
		fmt.Println("SSO client secret already created, retrieving")
		ou.secret = string(sateliteClientSecret)
	}

	respBytes, err := ou.clientset.RESTClient().Get().RequestURI("/apis/apiextensions.k8s.io/v1/customresourcedefinitions/openunisons.openunison.tremolo.io").DoRaw(context.TODO())
	if err != nil {
		return err
	}

	ouCrd := make(map[string]interface{})
	json.Unmarshal(respBytes, &ouCrd)

	spec := ouCrd["spec"].(map[string]interface{})
	versions, ok := spec["versions"].([]interface{})

	if !ok {
		return fmt.Errorf("no spec in openunison crd")
	}

	ouVersion := ""

	for _, v := range versions {
		version := v.(map[string]interface{})
		served := version["served"].(bool)
		stored := version["storage"].(bool)

		if served && stored {
			ouVersion = version["name"].(string)
		}
	}

	if ouVersion == "" {
		return fmt.Errorf("could not find version of openunisons")
	}

	fmt.Printf("OpenUnison CRD Version : %v\n", ouVersion)

	respBytes, err = ou.clientset.RESTClient().Get().RequestURI("/apis/openunison.tremolo.io/" + ouVersion + "/namespaces/" + ou.namespace + "/openunisons/orchestra").DoRaw(context.TODO())
	if err != nil {
		return err
	}

	orchestra := make(map[string]interface{})
	json.Unmarshal(respBytes, &orchestra)

	spec = orchestra["spec"].(map[string]interface{})
	hosts := spec["hosts"].([]interface{})
	host := hosts[0].(map[string]interface{})
	names := host["names"].([]interface{})

	idpHostName := ""

	for _, n := range names {
		name := n.(map[string]interface{})
		if name["env_var"].(string) == "OU_HOST" {
			idpHostName = name["name"].(string)
		}
	}

	if idpHostName == "" {
		return fmt.Errorf("could not find OU_HOST name in orchestra CRD")
	}

	fmt.Printf("Control Plane IdP host name: %v\n", idpHostName)

	keyStore := spec["key_store"].(map[string]interface{})
	keyPairs := keyStore["key_pairs"].(map[string]interface{})
	keys := keyPairs["keys"].([]interface{})

	isLocalGeneratedCert := false

	for _, k := range keys {
		key := k.(map[string]interface{})
		if key["name"].(string) == "unison-ca" {
			isLocalGeneratedCert = true
		}
	}

	fmt.Printf("Is the operator generating the TLS certificate? : %t\n", isLocalGeneratedCert)

	idpCert := ""

	if isLocalGeneratedCert {
		ouTlsKey, err := ou.clientset.CoreV1().Secrets(ou.namespace).Get(context.TODO(), "ou-tls-certificate", metav1.GetOptions{})

		if err != nil {
			return err
		}

		idpCert = string(ouTlsKey.Data["tls.crt"])
	} else {
		trustedCerts, ok := keyStore["trusted_certificates"].([]interface{})
		if ok {
			for _, c := range trustedCerts {
				cert := c.(map[string]interface{})
				if cert["name"] == "unison-ca" {
					bytes, err := base64.StdEncoding.DecodeString(cert["pem_data"].(string))
					if err != nil {
						return err
					}

					idpCert = string(bytes)
				}
			}
		}
	}

	fmt.Printf("IdP Certificate : %v\n", idpCert)

	// create updates to yaml

	// setup OIDC
	oidcConfig := make(map[string]interface{})

	oidcConfig["client_id"] = "cluster-idp-" + clusterName
	oidcConfig["issuer"] = "https://" + idpHostName + "/auth/idp/cluster-idp-" + clusterName
	oidcConfig["user_in_idtoken"] = true
	oidcConfig["domain"] = ""
	oidcConfig["scopes"] = "openid email profile groups"

	claims := make(map[string]string)
	claims["sub"] = "sub"
	claims["email"] = "email"
	claims["given_name"] = "given_name"
	claims["family_name"] = "family_name"
	claims["display_name"] = "display_name"
	claims["groups"] = "groups"

	oidcConfig["claims"] = claims

	helmValues["oidc"] = oidcConfig

	//add the idp's certificate
	if idpCert != "" {

		trustedCerts, ok := helmValues["trusted_certs"].([]interface{})
		if !ok {
			trustedCerts = make([]interface{}, 0)
		}

		foundCert := false

		b64Cert := base64.StdEncoding.EncodeToString([]byte(idpCert))

		for _, c := range trustedCerts {
			cert := c.(map[string]interface{})
			if cert["name"] == "trusted-idp" {
				cert["pem_b64"] = b64Cert
				foundCert = true
				break
			} else if cert["pem_b64"] == b64Cert {
				foundCert = true
				break
			}
		}

		if !foundCert {
			trustedCert := make(map[string]string)
			trustedCert["name"] = "trusted-idp"
			trustedCert["pem_b64"] = b64Cert

			trustedCerts = append(trustedCerts, trustedCert)

			helmValues["trusted_certs"] = trustedCerts
		}

	}

	dataToWrite, err := yaml.Marshal(&helmValues)

	if err != nil {
		return err
	}

	ioutil.WriteFile(ou.pathToValuesYaml, dataToWrite, 0)

	shouldReturn, returnValue := ou.integrateSatelite(helmValues, clusterName, err, sateliteIntegrated, actionConfig, satelateReleaseName, settings)
	if shouldReturn {
		return returnValue
	}

	// deploy the satelte
	fmt.Printf("Switching to %v\n", ou.satelateContextName)
	_, err = ou.setCurrentContext(ou.satelateContextName)

	if err != nil {
		return err
	}
	fmt.Printf("Deploying the satelite")
	ou.loadKubernetesConfiguration()
	err = ou.DeployAuthPortal()

	if err != nil {
		return err
	}

	// leave the kubeconfig the way we found it
	ou.setCurrentContext(originalContextName)

	fmt.Println(sateliteIntegrated)
	fmt.Println(originalContextName)

	return nil
}

func (ou *OpenUnisonDeployment) integrateSatelite(helmValues map[string]interface{}, clusterName string, err error, sateliteIntegrated bool, actionConfig *action.Configuration, satelateReleaseName string, settings *cli.EnvSettings) (bool, error) {
	cpYaml := `{
		"cluster": {
		  "name": "%v",
		  "label": "%v",
		  "description": "Cluster %v",
		  "sso": {
			"enabled": true,
			"inactivityTimeoutSeconds": 900
		  },
		  "hosts": {
			"portal": "%v",
			"dashboard": "%v"
		  },
		  "az_groups": [
	  
		  ]
		}
	  }`

	sateliteNetwork := helmValues["network"].(map[string]interface{})

	cpYaml = fmt.Sprintf(cpYaml,
		clusterName,
		clusterName,
		clusterName,
		sateliteNetwork["openunison_host"].(string),
		sateliteNetwork["dashboard_host"].(string))

	fmt.Printf("Integrating satelite into the control plane with yaml: \n%v\n", cpYaml)

	cpValues := make(map[string]interface{})
	err = json.Unmarshal([]byte(cpYaml), &cpValues)

	if err != nil {
		return true, err
	}

	if helmValues["openunison"].(map[string]interface{})["az_groups"] != nil {
		satelateAzGroups := helmValues["openunison"].(map[string]interface{})["az_groups"].([]interface{})
		cpAzGroups := make([]string, len(satelateAzGroups))

		for i, group := range satelateAzGroups {
			cpAzGroups[i] = string(group.(string))
		}

		cpValues["cluster"].(map[string]interface{})["az_groups"] = cpAzGroups
	}

	_, err = ou.setCurrentContext(ou.controlPlaneContextName)

	if err != nil {
		return true, err
	}

	err = ou.loadKubernetesConfiguration()
	if err != nil {
		return true, err
	}

	if !sateliteIntegrated {
		fmt.Print("Satelite not integrated yet, deploying")
		client := action.NewInstall(actionConfig)

		client.Namespace = ou.namespace
		client.ReleaseName = satelateReleaseName

		cp, err := client.ChartPathOptions.LocateChart(ou.addClusterChart, settings)

		if err != nil {
			return true, err
		}

		chartReq, err := loader.Load(cp)

		if err != nil {
			return true, err
		}

		_, err = client.Run(chartReq, cpValues)

		if err != nil {
			return true, err
		}
	} else {
		fmt.Println("Satelite already integrated, upgrading")
		client := action.NewUpgrade(actionConfig)

		client.Namespace = ou.namespace

		cp, err := client.ChartPathOptions.LocateChart(ou.addClusterChart, settings)

		if err != nil {
			return true, err
		}

		chartReq, err := loader.Load(cp)

		if err != nil {
			return true, err
		}

		_, err = client.Run(satelateReleaseName, chartReq, cpValues)

		if err != nil {
			return true, err
		}
	}
	return false, nil
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

		hasSecret := ou.secretFile != "" || ou.secret != ""

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
			authSecretLabel = "OIDC_CLIENT_SECRET"
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
	fmt.Printf("Waiting for a few seconds for the operator to run")
	time.Sleep(5 * time.Second)
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

		labels := ""

		for key, value := range dep.Spec.Selector.MatchLabels {
			labels = labels + key + "=" + value + ","
		}

		labels = labels[0 : len(labels)-1]
		fmt.Printf("Looking for labels '%v'\n", labels)
		options := metav1.ListOptions{
			LabelSelector: labels,
		}

		pods, err := ou.clientset.CoreV1().Pods(ou.namespace).List(context.TODO(), options)

		if err != nil {
			return err
		}

		numPods := len(pods.Items)

		fmt.Printf("Total Pods : %v, Ready Pods : %v\n", numPods, dep.Status.ReadyReplicas)

		if *dep.Spec.Replicas <= dep.Status.ReadyReplicas && int32(numPods) == *dep.Spec.Replicas {
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
