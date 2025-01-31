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
	"path/filepath"
	"strings"
	"time"

	"github.com/tremolosecurity/openunison-control/helmmodel"
	"github.com/tremolosecurity/openunison-control/openunisonmodel"
	"gopkg.in/yaml.v3"

	v1 "k8s.io/api/core/v1"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"

	"helm.sh/helm/v3/pkg/registry"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
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
	chart string
}

// for sting info about additional helm charts
type HelmChartInfo struct {
	Name      string
	ChartPath string
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

	clusterManagementChart string
	pathToDbPassword       string
	pathToSmtpPassword     string

	pathToSaveSateliteValues string

	skipClusterManagement bool

	helmValues map[string]interface{}

	additionalCharts []HelmChartInfo
	preCharts        []HelmChartInfo

	namespaceLabels map[string]string

	cpOrchestraName string
	cpSecretName    string

	skipCpIntegration bool
}

// creates a new deployment structure
func NewOpenUnisonDeployment(namespace string, operatorChart string, orchestraChart string, orchestraLoginPortalChart string, pathToValuesYaml string, secretFile string, clusterManagementChart string, pathToDbPassword string, pathToSmtpPassword string, skipClusterManagement bool, additionalCharts []HelmChartInfo, preCharts []HelmChartInfo, namespaceLabels map[string]string) (*OpenUnisonDeployment, error) {
	ou, err := NewSateliteDeployment(namespace, operatorChart, orchestraChart, orchestraLoginPortalChart, pathToValuesYaml, secretFile, "", "", "", "", additionalCharts, preCharts, namespaceLabels, "orchestra", "orchestra-secrets-source", false)

	if err != nil {
		return nil, err
	}

	ou.clusterManagementChart = clusterManagementChart
	ou.pathToDbPassword = pathToDbPassword
	ou.pathToSmtpPassword = pathToSmtpPassword
	ou.skipClusterManagement = skipClusterManagement

	return ou, nil
}

// creates a new deployment structure
func NewSateliteDeployment(namespace string, operatorChart string, orchestraChart string, orchestraLoginPortalChart string, pathToValuesYaml string, secretFile string, controlPlanContextName string, sateliteContextName string, addClusterChart string, pathToSateliteYaml string, additionalCharts []HelmChartInfo, preCharts []HelmChartInfo, namespaceLabels map[string]string, cpOrchestraName string, cpSecretName string, skipCpIntegration bool) (*OpenUnisonDeployment, error) {
	ou := &OpenUnisonDeployment{}

	ou.namespace = namespace

	ou.operator.chart = operatorChart

	ou.orchestraChart = orchestraChart
	ou.orchestraLoginPortalChart = orchestraLoginPortalChart
	ou.pathToValuesYaml = pathToValuesYaml
	ou.secretFile = secretFile

	ou.controlPlaneContextName = controlPlanContextName
	ou.satelateContextName = sateliteContextName
	ou.addClusterChart = addClusterChart

	ou.pathToSaveSateliteValues = pathToSateliteYaml

	ou.additionalCharts = additionalCharts
	ou.preCharts = preCharts

	err := ou.loadKubernetesConfiguration()
	if err != nil {
		return nil, err
	}

	err = ou.loadHelmValues()

	if err != nil {
		return nil, err
	}

	ou.namespaceLabels = namespaceLabels

	ou.cpOrchestraName = cpOrchestraName
	ou.cpSecretName = cpSecretName
	ou.skipCpIntegration = skipCpIntegration

	return ou, nil
}

func (ou *OpenUnisonDeployment) loadHelmValues() error {
	fmt.Printf("Loading values from %s...\n", ou.pathToValuesYaml)

	yamlValues, err := ioutil.ReadFile(ou.pathToValuesYaml)

	if err != nil {
		return err
	}

	ou.helmValues = make(map[string]interface{})

	err = yaml.Unmarshal(yamlValues, &ou.helmValues)

	if err != nil {
		return err
	}

	fmt.Printf("...loaded\n")

	return nil
}

func (ou *OpenUnisonDeployment) IsNaas() bool {
	return isNaasFromHelm(ou.helmValues)
}

func isNaasFromHelm(helm map[string]interface{}) bool {
	openunison, ok := helm["openunison"].(map[string]interface{})

	if !ok {
		return false
	}

	enableProvisioning := openunison["enable_provisioning"].(bool)

	return enableProvisioning
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

// deploy a NaaS Portal

func (ou *OpenUnisonDeployment) DeployNaaSPortal() error {

	openunison := ou.helmValues["openunison"].(map[string]interface{})
	enableProvisioning := openunison["enable_provisioning"].(bool)

	if !enableProvisioning {
		return fmt.Errorf("openunison.enableProvisioning MUST be true")
	}

	externalEnabled := false
	internalEnabled := false

	externalSuffix := ""
	internalSuffix := ""

	naas, found := openunison["naas"].(map[string]interface{})
	if found {
		groups, found := naas["groups"].(map[string]interface{})
		if found {
			external, found := groups["external"].(map[string]interface{})
			if found {
				externalEnabled, found = external["enabled"].(bool)
			}
		}
	}

	useStdJit, found := openunison["use_standard_jit_workflow"].(bool)

	if !found {
		useStdJit = true
	}

	if externalEnabled && useStdJit {
		return fmt.Errorf("openunison.naas.groups.external is true, openunison.use_standard_jit_workflow MUST be false")
	}

	_, ok := ou.helmValues["database"]

	if !ok {
		return fmt.Errorf("no database section to your values.yaml")
	}

	_, ok = ou.helmValues["smtp"]

	if !ok {
		return fmt.Errorf("no smtp section to your values.yaml")
	}

	// deploy the operator
	settings := cli.New()
	actionConfig := new(action.Configuration)

	if err := actionConfig.Init(settings.RESTClientGetter(), ou.namespace, os.Getenv("HELM_DRIVER"), log.Printf); err != nil {
		return err
	}

	listClient := action.NewList(actionConfig)

	clusterManagementChartDeployed := false

	listClient.All = true
	releases, err := listClient.Run()

	for _, release := range releases {
		if release.Name == "cluster-management" && release.Namespace == ou.namespace {
			clusterManagementChartDeployed = true
		}
	}

	// add standard groups to NaaS based on roles defined
	client := action.NewInstall(actionConfig)

	client.Namespace = ou.namespace
	client.ReleaseName = "cluster-management"

	chartReq, err := ou.locateChart(ou.clusterManagementChart, &client.ChartPathOptions, settings)

	if err != nil {
		return err
	}

	mergedValues := mergeMaps(chartReq.Values, ou.helmValues)

	openunisonCfg, ok := mergedValues["openunison"]

	if !ok {
		return fmt.Errorf("When configuring NaaS groups, no openunison section")
	}

	naasCfg, ok := openunisonCfg.(map[string]interface{})["naas"]

	if !ok {
		return fmt.Errorf("When configuring NaaS groups, no openunison.naas section")
	}

	groupsCfg, ok := naasCfg.(map[string]interface{})["groups"]
	if !ok {
		return fmt.Errorf("When configuring NaaS groups, no openunison.naas.groups section")
	}

	external, found := groupsCfg.(map[string]interface{})["external"].(map[string]interface{})
	if found {
		externalEnabled, found = external["enabled"].(bool)
		if externalEnabled {
			externalSuffix = external["suffix"].(string)
		}
	}

	internal, found := groupsCfg.(map[string]interface{})["internal"].(map[string]interface{})
	if found {
		internalEnabled, found = internal["enabled"].(bool)
		if internalEnabled {
			internalSuffix = internal["suffix"].(string)
		}
	}

	defaultGroups, ok := groupsCfg.(map[string]interface{})["default"]
	if !ok {
		return fmt.Errorf("When configuring NaaS groups, no openunison.naas.groups.default section")
	}

	naasRoles := make([]string, 0)

	for _, group := range defaultGroups.([]interface{}) {
		groupName := group.(map[string]interface{})["name"].(string)
		naasRoles = append(naasRoles, groupName)
	}

	azRules := make([]interface{}, len(naasRoles))

	for i, role := range naasRoles {
		azRules[i] = fmt.Sprintf("k8s-namespace-%v-k8s-%v-*", role, "k8s")
	}

	if internalEnabled {
		azRules = append(azRules, fmt.Sprintf("k8s-cluster-k8s-%v-administrators%v", "k8s", internalSuffix))
	}

	if externalEnabled {
		azRules = append(azRules, fmt.Sprintf("k8s-cluster-k8s-%v-administrators%v", "k8s", externalSuffix))
	}

	ou.helmValues["openunison"].(map[string]interface{})["az_groups"] = azRules

	// with merged values, create azRules

	err = ou.DeployAuthPortal()

	if err != nil {
		return err
	}

	//if !openunisonDeployed {
	fmt.Print("Deploying the Cluster Management chart\n")

	if ou.skipClusterManagement {
		fmt.Println("Skipping cluster management chart")
	} else {

		if !clusterManagementChartDeployed {
			fmt.Println("Chart not deployed, installing")
			client := action.NewInstall(actionConfig)

			client.Namespace = ou.namespace
			client.ReleaseName = "cluster-management"

			chartReq, err := ou.locateChart(ou.clusterManagementChart, &client.ChartPathOptions, settings)
			if err != nil {
				return err
			}

			mergedValues := mergeMaps(chartReq.Values, ou.helmValues)

			//_, err = client.Run(chartReq, mergedValues)
			_, err = ou.runChartInstall(client, client.ReleaseName, chartReq, mergedValues, actionConfig)

			if err != nil {
				return err
			}
		} else {
			fmt.Println("Chart deployed, upgrading")
			client := action.NewUpgrade(actionConfig)

			client.Namespace = ou.namespace

			chartReq, err := ou.locateChart(ou.clusterManagementChart, &client.ChartPathOptions, settings)

			if err != nil {
				return err
			}

			mergedValues := mergeMaps(chartReq.Values, ou.helmValues)
			//_, err = client.Run("cluster-management", chartReq, mergedValues)
			_, err = ou.runChartUpgrade(client, "cluster-management", chartReq, mergedValues)

			if err != nil {
				return err
			}
		}
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

	// get the satelite cluster name

	clusterName, ok := ou.helmValues["k8s_cluster_name"].(string)

	if !ok {
		return fmt.Errorf("k8s_cluster_name must be defined in the satalite values.yaml")
	}

	satelateReleaseName := "satellite-" + clusterName

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

	ouSecret, err := ou.clientset.CoreV1().Secrets(ou.namespace).Get(context.TODO(), ou.cpSecretName, metav1.GetOptions{})
	foundSecret := false
	if err != nil {
		ouSecret = &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ou.cpSecretName,
				Namespace: ou.namespace,
			},
			Data: map[string][]byte{},
		}
	} else {
		foundSecret = true
	}

	sateliteClientSecret, ok := ouSecret.Data["cluster-idp-"+clusterName]

	if !ok {
		fmt.Println("SSO Client Secret doesn't exist, creating")
		ou.secret = string(randSeq((64)))
		ouSecret.Data["cluster-idp-"+clusterName] = []byte(ou.secret)

		if foundSecret {
			ou.clientset.CoreV1().Secrets(ou.namespace).Update(context.TODO(), ouSecret, metav1.UpdateOptions{})
		} else {
			_, err = ou.clientset.CoreV1().Secrets(ou.namespace).Create(context.TODO(), ouSecret, metav1.CreateOptions{})
			if err != nil {
				return err
			}
		}

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

	respBytes, err = ou.clientset.RESTClient().Get().RequestURI("/apis/openunison.tremolo.io/" + ouVersion + "/namespaces/" + ou.namespace + "/openunisons/" + ou.cpOrchestraName).DoRaw(context.TODO())
	if err != nil {
		return err
	}

	orchestra := make(map[string]interface{})
	json.Unmarshal(respBytes, &orchestra)

	var orchestraObj openunisonmodel.OpenUnison
	json.Unmarshal(respBytes, &orchestraObj)

	specObj := orchestraObj.Spec
	hosts := specObj.Hosts
	host := hosts[0]
	names := host.Names

	nonSecretData := specObj.NonSecretData

	naasEnabled := false
	naasGroupsInternal := false
	naasGroupsExternal := false

	naasInternalSuffix := ""
	naasExternalSuffix := ""

	naasRoles := make([]map[string]interface{}, 0)

	for _, nsd := range nonSecretData {

		nsdName := nsd.Name

		if nsdName == "OPENUNISON_PROVISIONING_ENABLED" {
			naasEnabled = nsd.Value == "true"
		} else if nsdName == "openunison.naas.external" {
			naasGroupsExternal = nsd.Value == "true"
		} else if nsdName == "openunison.naas.internal" {
			naasGroupsInternal = nsd.Value == "true"
		} else if nsdName == "openunison.naas.external-suffix" {
			naasExternalSuffix = nsd.Value
		} else if nsdName == "openunison.naas.internal-suffix" {
			naasInternalSuffix = nsd.Value
		} else if nsdName == "openunison.naas.default-groups" {
			enc, err := base64.StdEncoding.DecodeString(nsd.Value)
			if err != nil {
				return err
			}

			var localRoles []map[string]interface{}

			err = json.Unmarshal(enc, &localRoles)

			if err != nil {
				return err
			}

			naasRoles = append(naasRoles, localRoles...)

		} else if nsdName == "openunison.naas.roles" {
			enc, err := base64.StdEncoding.DecodeString(nsd.Value)
			if err != nil {
				return err
			}

			var localRoles []map[string]interface{}

			err = json.Unmarshal(enc, &localRoles)

			if err != nil {
				return err
			}

			naasRoles = append(naasRoles, localRoles...)

		}

	}

	idpHostName := ""

	for _, name := range names {

		if name.EnvVar == "OU_HOST" {
			idpHostName = name.Name
		}
	}

	if idpHostName == "" {
		return fmt.Errorf("could not find OU_HOST name in orchestra CRD")
	}

	fmt.Printf("Control Plane IdP host name: %v\n", idpHostName)

	keyStore := specObj.KeyStore
	keyPairs := keyStore.KeyPairs
	keys := keyPairs.Keys

	isLocalGeneratedCert := false

	for _, key := range keys {

		if key.Name == "unison-ca" {
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
		trustedCerts := keyStore.TrustedCertificates

		for _, cert := range trustedCerts {

			if cert.Name == "unison-ca" {
				bytes, err := base64.StdEncoding.DecodeString(cert.PemData)
				if err != nil {
					return err
				}

				idpCert = string(bytes)
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

	ou.helmValues["oidc"] = oidcConfig

	trustedCertAlias := "trusted-idp"

	//add the idp's certificate
	if idpCert != "" {

		trustCertsJson, err := json.Marshal(ou.helmValues["trusted_certs"])

		if err != nil {
			panic(err)
		}

		var trustedCerts []helmmodel.TrustedCertsInner

		json.Unmarshal(trustCertsJson, &trustedCerts)

		// trustedCerts, ok := ou.helmValues["trusted_certs"].([]interface{})
		// if !ok {
		// 	trustedCerts = make([]interface{}, 0)
		// }

		foundCert := false

		b64Cert := base64.StdEncoding.EncodeToString([]byte(idpCert))

		for _, cert := range trustedCerts {

			if cert.Name == "trusted-idp" {
				cert.PemB64 = b64Cert
				foundCert = true
				break
			} else if cert.PemB64 == b64Cert {
				foundCert = true
				trustedCertAlias = cert.Name
				break
			}
		}

		if !foundCert {
			trustedCert := &helmmodel.TrustedCertsInner{}
			trustedCert.Name = "trusted-idp"
			trustedCert.PemB64 = b64Cert

			trustedCerts = append(trustedCerts, *trustedCert)

			tcjson, err := json.Marshal(trustedCerts)

			if err != nil {
				panic(err)
			}

			var tcarray []map[string]string
			json.Unmarshal(tcjson, &tcarray)

			ou.helmValues["trusted_certs"] = tcarray
		}

	}

	managementProxyUrl := ""
	externalNaasGroupName := ""
	sateliteManagementEnabled := false
	mgmtProxy := make(map[string]interface{})

	if naasEnabled {
		openunison := ou.helmValues["openunison"].(map[string]interface{})
		mgmtProxy, sateliteManagementEnabled = openunison["management_proxy"].(map[string]interface{})

		if sateliteManagementEnabled {
			fmt.Printf("Management proxy enabled\n")
			mgmtEnabled, ok := mgmtProxy["enabled"]
			if ok && mgmtEnabled == true {
				// set remote configuration
				managementProxyUrl = mgmtProxy["host"].(string)
				remote := make(map[string]string)
				remote["issuer"] = fmt.Sprintf("https://%v/auth/idp/remotek8s", idpHostName)
				if idpCert != "" {
					remote["cert_alias"] = trustedCertAlias
				}
				mgmtProxy["remote"] = remote

				azRules := make([]interface{}, len(naasRoles))

				for i, role := range naasRoles {
					azRules[i] = fmt.Sprintf("k8s-namespace-%v-k8s-%v-*", role["name"].(string), clusterName)
				}

				if naasGroupsInternal {
					azRules = append(azRules, fmt.Sprintf("k8s-cluster-k8s-%v-administrators%v", clusterName, naasInternalSuffix))
				}

				if naasGroupsExternal {
					azRules = append(azRules, fmt.Sprintf("k8s-cluster-k8s-%v-administrators%v", clusterName, naasExternalSuffix))
					externalNaasGroupName = mgmtProxy["external_admin_group"].(string)
					mgmtProxy["external_suffix"] = naasExternalSuffix
				}

				// if there are pre-set az_groups, they should be honored

				if openunison["az_groups"] != nil {
					sateliteAzGroups := openunison["az_groups"].([]interface{})
					openunison["az_groups"] = append(sateliteAzGroups, azRules...)
				} else {
					openunison["az_groups"] = azRules
				}

			}
		}
	}

	dataToWrite, err := yaml.Marshal(&ou.helmValues)

	if err != nil {
		return err
	}

	ioutil.WriteFile(ou.pathToValuesYaml, dataToWrite, 0644)

	if !ou.skipCpIntegration {
		shouldReturn, returnValue := ou.integrateSatelite(ou.helmValues, clusterName, err, sateliteIntegrated, actionConfig, satelateReleaseName, settings, nil, "", "", naasRoles)
		if shouldReturn {
			return returnValue
		}
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

	err = ou.DeployAdditionalCharts()
	if err != nil {
		return err
	}

	if !ou.skipCpIntegration {
		if naasEnabled && sateliteManagementEnabled {
			// if the naas is enabled, need to deploy management
			targetCert := ""
			var trustedCerts []helmmodel.TrustedCertsInner
			trustCertsJson, err := json.Marshal(ou.helmValues["trusted_certs"])

			if err != nil {
				panic(err)
			}

			json.Unmarshal(trustCertsJson, &trustedCerts)

			for _, trustedCert := range trustedCerts {
				certName := trustedCert.Name
				if certName == "unison-ca" {
					targetCert = trustedCert.PemB64
				}
			}

			// trustedCerts, ok := ou.helmValues["trusted_certs"].([]interface{})
			// if ok {
			// 	for _, t := range trustedCerts {
			// 		trustedCert := t.(map[string]interface{})
			// 		certName := trustedCert["name"].(string)
			// 		if certName == "unison-ca" {
			// 			targetCert = trustedCert["pem_b64"].(string)
			// 		}
			// 	}
			// }

			if targetCert == "" {
				// not found, load the ou-tls-certificate secret
				tlsSecret, err := ou.clientset.CoreV1().Secrets(ou.namespace).Get(context.TODO(), "ou-tls-certificate", metav1.GetOptions{})
				if err == nil {
					targetCert = base64.StdEncoding.EncodeToString(tlsSecret.Data["tls.crt"])
				}
			}

			management := make(map[string]interface{})
			management["enabled"] = true
			target := make(map[string]interface{})
			management["target"] = target

			target["url"] = managementProxyUrl
			target["tokenType"] = "oidc"
			target["useToken"] = true

			if targetCert != "" {
				target["base64_certificate"] = targetCert
			}

			// redeployment satelite integration
			ou.setCurrentContext(ou.controlPlaneContextName)
			ou.loadKubernetesConfiguration()
			shouldReturn, returnValue := ou.integrateSatelite(ou.helmValues, clusterName, err, sateliteIntegrated, actionConfig, satelateReleaseName, settings, management, naasExternalSuffix, externalNaasGroupName, naasRoles)
			if shouldReturn {
				return returnValue
			}

		}

	}

	// leave the kubeconfig the way we found it
	ou.setCurrentContext(originalContextName)

	fmt.Println(sateliteIntegrated)
	fmt.Println(originalContextName)
	return nil
}

func (ou *OpenUnisonDeployment) integrateSatelite(helmValues map[string]interface{}, clusterName string, err error, sateliteIntegrated bool, actionConfig *action.Configuration, satelateReleaseName string, settings *cli.EnvSettings, management map[string]interface{}, externalGroupNameSuffix string, externalGroupName string, naasRoles []map[string]interface{}) (bool, error) {
	cpYaml := `{
		"cluster": {
		  "name": "%v",
		  "label": "%v",
		  "description": "Cluster %v",
		  "parent": "%v",
		  "sso": {
			"enabled": true,
			"inactivityTimeoutSeconds": 900,
			"client_secret": "%v"
		  },
		  "hosts": {
			"portal": "%v",
			"dashboard": "%v"
		  },
		  "az_groups": [
	  
		  ]
		  
		}
	  }`

	parentOrg := ""

	openunison := helmValues["openunison"].(map[string]interface{})

	if openunison != nil {

		if openunison["control_plane"] != nil {
			controlPlane := openunison["control_plane"].(map[string]interface{})
			parentOrg = controlPlane["parent"].(string)
		}
	}

	if parentOrg == "" {
		parentOrg = "B158BD40-0C1B-11E3-8FFD-0800200C9A66"
	}

	sateliteNetwork := helmValues["network"].(map[string]interface{})

	cpYaml = fmt.Sprintf(cpYaml,
		clusterName,
		clusterName,
		clusterName,
		parentOrg,
		ou.cpSecretName,
		sateliteNetwork["openunison_host"].(string),
		sateliteNetwork["dashboard_host"].(string))

	fmt.Printf("Integrating satelite into the control plane with yaml: \n%v\n", cpYaml)

	cpValues := make(map[string]interface{})
	err = json.Unmarshal([]byte(cpYaml), &cpValues)

	if err != nil {
		return true, err
	}

	if management != nil {
		cluster := cpValues["cluster"].(map[string]interface{})
		cluster["management"] = management

		if externalGroupName != "" {
			cluster["external_group_name"] = externalGroupName
			cluster["external_group_name_suffix"] = externalGroupNameSuffix
		}
	}

	if helmValues["openunison"].(map[string]interface{})["az_groups"] != nil {
		satelateAzGroups := helmValues["openunison"].(map[string]interface{})["az_groups"].([]interface{})
		cpAzGroups := make([]string, len(satelateAzGroups))

		for i, group := range satelateAzGroups {
			cpAzGroups[i] = string(group.(string))
		}

		cpValues["cluster"].(map[string]interface{})["az_groups"] = cpAzGroups
	}

	if openunison != nil {

		if openunison["control_plane"] != nil {
			controlPlane := openunison["control_plane"].(map[string]interface{})
			additionalBadges := controlPlane["additional_badges"].([]interface{})
			if additionalBadges != nil {
				cpValues["cluster"].(map[string]interface{})["additional_badges"] = additionalBadges
			}
		}
	}

	cpValues["naasRoles"] = naasRoles

	if ou.pathToSaveSateliteValues != "" {
		dataToWrite, err := yaml.Marshal(&cpValues)

		if err != nil {
			return false, err
		}

		ioutil.WriteFile(ou.pathToSaveSateliteValues, dataToWrite, 0644)
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

		chartReq, err := ou.locateChart(ou.addClusterChart, &client.ChartPathOptions, settings)

		if err != nil {
			return true, err
		}

		//_, err = client.Run(chartReq, cpValues)
		_, err = ou.runChartInstall(client, client.ReleaseName, chartReq, cpValues, actionConfig)
		if err != nil {
			return true, err
		}
	} else {
		fmt.Println("Satelite already integrated, upgrading")
		client := action.NewUpgrade(actionConfig)

		client.Namespace = ou.namespace

		chartReq, err := ou.locateChart(ou.addClusterChart, &client.ChartPathOptions, settings)

		if err != nil {
			return true, err
		}

		//_, err = client.Run(satelateReleaseName, chartReq, cpValues)
		_, err = ou.runChartUpgrade(client, satelateReleaseName, chartReq, cpValues)
		if err != nil {
			return true, err
		}
	}

	return false, nil
}

func (ou *OpenUnisonDeployment) runChartInstall(client *action.Install, name string, chartReq *chart.Chart, cpValues map[string]interface{}, actionConfig *action.Configuration) (bool, error) {
	for i := 0; i <= 5; i++ {
		_, err := client.Run(chartReq, cpValues)
		if err != nil {
			fmt.Printf("Error installing chart %s - %s, deleting and retrying\n", name, err.Error())

			del := action.NewUninstall(actionConfig)
			_, err := del.Run(name)
			if err != nil {
				return true, err
			}
			fmt.Println("Waiting a few seconds...")
			time.Sleep(5 * time.Second)
			fmt.Printf("Try #%i\n", i)
		} else {
			return false, nil
		}
	}

	return true, fmt.Errorf("Failed to install chart %s after five tries", name)
}

func (ou *OpenUnisonDeployment) runChartUpgrade(client *action.Upgrade, name string, chartReq *chart.Chart, cpValues map[string]interface{}) (bool, error) {
	for i := 0; i <= 5; i++ {
		_, err := client.Run(name, chartReq, cpValues)
		if err != nil {
			fmt.Printf("Error installing chart %s - %s, retrying\n", name, err.Error())

			fmt.Println("Waiting a few seconds...")
			time.Sleep(5 * time.Second)
			fmt.Printf("Try #%i\n", i)
		} else {
			return false, nil
		}
	}

	return true, fmt.Errorf("Failed to install chart %s after five tries", name)
}

// set the secret
func (ou *OpenUnisonDeployment) setupSecret(helmValues map[string]interface{}) error {
	if ou.skipCpIntegration {
		return nil
	}

	secret, err := ou.clientset.CoreV1().Secrets(ou.namespace).Get(context.TODO(), "orchestra-secrets-source", metav1.GetOptions{})
	foundSecret := false
	if err != nil {
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
	} else {
		foundSecret = true
	}

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

			authSecret = []byte(strings.TrimSpace(string(authSecret)))
		}

	}

	authNeedsSecret := false
	authSecretLabel := ""
	foundAuth := false

	_, ok := helmValues["oidc"]

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

	_, ok = secret.Data[authSecretLabel]

	if !ok {
		// there's not already a secret
		if authNeedsSecret && !hasSecret {
			return fmt.Errorf("Authentication type requires a secret in a file specified in -s or --secrets-file-path")
		}
	}

	if authNeedsSecret && hasSecret {
		secret.Data[authSecretLabel] = authSecret
	}

	openunison := helmValues["openunison"].(map[string]interface{})
	enableNaaS := openunison["enable_provisioning"].(bool)

	if enableNaaS {
		//check for the database password
		_, hasJdbcPassword := secret.Data["OU_JDBC_PASSWORD"]
		if ou.pathToDbPassword != "" {
			dbSecret, err := ioutil.ReadFile(ou.pathToDbPassword)
			if err != nil {
				return err
			}

			dbSecret = []byte(strings.TrimSpace(string(dbSecret)))

			fmt.Println("Setting database password\n")

			secret.Data["OU_JDBC_PASSWORD"] = dbSecret
		} else if !hasJdbcPassword {
			return fmt.Errorf("if openunison.enable_provisioning is true, -b or --database-secret-path must be set")
		}

		// check for SMTP
		_, hasSmtpPassword := secret.Data["SMTP_PASSWORD"]
		if ou.pathToSmtpPassword != "" {
			smtpSecret, err := ioutil.ReadFile(ou.pathToSmtpPassword)
			if err != nil {
				return err
			}

			smtpSecret = []byte(strings.TrimSpace(string(smtpSecret)))

			fmt.Println("Setting SMTP password")
			secret.Data["SMTP_PASSWORD"] = smtpSecret
		} else if !hasSmtpPassword {
			return fmt.Errorf("if openunison.enable_provisioning is true, -t or --smtp-secret-path must be set")
		}
	}

	if !foundSecret {
		fmt.Printf("Creating secret\n")
		secret, err = ou.clientset.CoreV1().Secrets(ou.namespace).Create(context.TODO(), secret, metav1.CreateOptions{})
		if err != nil {
			return err
		}
	} else {
		fmt.Printf("Updating secret\n")
		secret, err = ou.clientset.CoreV1().Secrets(ou.namespace).Update(context.TODO(), secret, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}

	return nil

}

// deploys all extra charts
func (ou *OpenUnisonDeployment) DeployAdditionalCharts() error {
	fmt.Printf("Deploying additional charts: %d\n", len(ou.additionalCharts))
	for _, chart := range ou.additionalCharts {
		err := ou.deployChart(chart)
		if err != nil {
			return err
		}
	}

	return nil
}

// deploys all extra charts
func (ou *OpenUnisonDeployment) DeployPreCharts() error {
	for _, chart := range ou.preCharts {
		err := ou.deployChart(chart)
		if err != nil {
			return err
		}
	}

	return nil
}

// deploys additional charts after OpenUnison is running
func (ou *OpenUnisonDeployment) deployChart(chart HelmChartInfo) error {
	fmt.Printf("Deploying chart %s, %s\n", chart.Name, chart.ChartPath)

	settings := cli.New()
	actionConfig := new(action.Configuration)

	if err := actionConfig.Init(settings.RESTClientGetter(), ou.namespace, os.Getenv("HELM_DRIVER"), log.Printf); err != nil {
		return err
	}

	listClient := action.NewList(actionConfig)

	found := false

	listClient.All = true
	listClient.Failed = true
	releases, err := listClient.Run()

	if err != nil {
		return err
	}

	for _, release := range releases {
		if release.Name == chart.Name && release.Namespace == ou.namespace {
			found = true
		}
	}

	if !found {
		fmt.Println("Chart not deployed, installing")
		client := action.NewInstall(actionConfig)

		client.Namespace = ou.namespace
		client.ReleaseName = chart.Name

		chartReq, err := ou.locateChart(chart.ChartPath, &client.ChartPathOptions, settings)

		if err != nil {
			return err
		}

		//_, err = client.Run(chartReq, ou.helmValues)
		_, err = ou.runChartInstall(client, client.ReleaseName, chartReq, ou.helmValues, actionConfig)

		if err != nil {
			return err
		}

	} else {
		fmt.Println("Chart deployed, upgrading")
		client := action.NewUpgrade(actionConfig)

		client.Namespace = ou.namespace

		chartReq, err := ou.locateChart(chart.ChartPath, &client.ChartPathOptions, settings)

		if err != nil {
			return err
		}

		//_, err = client.Run(chart.Name, chartReq, ou.helmValues)

		_, err = ou.runChartUpgrade(client, chart.Name, chartReq, ou.helmValues)

		if err != nil {
			return err
		}
	}

	fmt.Printf("Chart %s, %s deployed\n", chart.Name, chart.ChartPath)

	return nil

}

// specifies chart version

func (ou *OpenUnisonDeployment) locateChart(configChartName string, chartPathOptions *action.ChartPathOptions, settings *cli.EnvSettings) (*chart.Chart, error) {
	chartName := configChartName
	chartVersion := ""
	if strings.Contains(configChartName, "@") {
		chartName = configChartName[0:strings.Index(configChartName, "@")]
		chartVersion = configChartName[strings.Index(configChartName, "@")+1:]

		fmt.Printf("Chart version specified for %s: %s\n", chartName, chartVersion)

		chartPathOptions.Version = chartVersion

	} else {
		fmt.Printf("No chart version specified for %s\n", chartName)
	}

	// Check if the chart is using OCI
	if strings.HasPrefix(chartName, "oci://") {
		fmt.Printf("OCI chart detected: %s\n", chartName)

		// Step 1: Initialize OCI client
		ociClient, err := registry.NewClient(
			registry.ClientOptEnableCache(true),
			registry.ClientOptDebug(true),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize OCI client: %v", err)
		}

		// Step 2: Create a temporary directory
		tempDir, err := os.MkdirTemp("", "helm-oci-*")
		if err != nil {
			return nil, fmt.Errorf("failed to create temporary directory: %v", err)
		}
		fmt.Printf("Temporary directory created: %s\n", tempDir)

		debugLog := func(format string, v ...interface{}) {
			fmt.Printf(format+"\n", v...)
		}

		actionConfig := new(action.Configuration)
		if err := actionConfig.Init(settings.RESTClientGetter(), "", os.Getenv("HELM_DRIVER"), debugLog); err != nil {
			return nil, fmt.Errorf("failed to initialize Helm action configuration: %v", err)
		}

		actionConfig.RegistryClient = ociClient

		// Step 3: Pull the chart from the OCI registry
		pull := action.NewPullWithOpts(action.WithConfig(actionConfig))
		pull.DestDir = tempDir
		pull.Version = chartVersion
		pull.Settings = &cli.EnvSettings{}

		_, err = pull.Run(chartName)
		if err != nil {
			return nil, fmt.Errorf("failed to pull OCI chart %s: %v", chartName, err)
		}

		// Step 4: Load the chart from the temporary directory
		chartFilePath := filepath.Join(tempDir, filepath.Base(chartName)+"-"+chartVersion+".tgz")
		chartReq, err := loader.Load(chartFilePath)
		if err != nil {
			return nil, fmt.Errorf("failed to load OCI chart from %s: %v", chartFilePath, err)
		}

		return chartReq, nil
	}

	cp, err := chartPathOptions.LocateChart(chartName, settings)

	if err != nil {
		return nil, err
	}

	chartReq, err := loader.Load(cp)

	if err != nil {
		return nil, err
	}

	return chartReq, nil

}

// deploys OpenUnison into the cluster
func (ou *OpenUnisonDeployment) DeployAuthPortal() error {
	// check the kubernetes dashboard ns exists

	dashboardNamespace := "kubernetes-dashboard"

	dashboardConfig, ok := ou.helmValues["dashboard"].(map[interface{}]interface{})
	if ok {

		dashboardNamespace = dashboardConfig["namespace"].(string)
	}

	ou.checkNamespace("Dashboard", dashboardNamespace)

	// check the openunison namespace exists, if not, create it

	ou.checkNamespace("OpenUnison", ou.namespace)

	network := ou.helmValues["network"].(map[string]interface{})
	ingressType := network["ingress_type"].(string)

	if ingressType == "istio" {
		fmt.Println("Enabling Istio on the openunison namespace")
		ouNs, err := ou.clientset.CoreV1().Namespaces().Get(context.TODO(), ou.namespace, metav1.GetOptions{})
		if err != nil {
			return err
		}

		ouNs.Labels["istio-injection"] = "enabled"
		ou.clientset.CoreV1().Namespaces().Update(context.TODO(), ouNs, metav1.UpdateOptions{})

	}

	err := ou.setupSecret(ou.helmValues)

	if err != nil {
		return err
	}

	// run pre-charts
	err = ou.DeployPreCharts()

	if err != nil {
		return err
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
	listClient.Failed = true
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

		chartReq, err := ou.locateChart(ou.operator.chart, &client.ChartPathOptions, settings)

		if err != nil {
			return err
		}

		//_, err = client.Run(chartReq, vals)
		_, err = ou.runChartInstall(client, client.ReleaseName, chartReq, ou.helmValues, actionConfig)

		if err != nil {
			return err
		}
	} else {
		fmt.Println("Chart deployed, upgrading")
		client := action.NewUpgrade(actionConfig)

		client.Namespace = ou.namespace

		chartReq, err := ou.locateChart(ou.operator.chart, &client.ChartPathOptions, settings)

		if err != nil {
			return err
		}

		_, err = ou.runChartUpgrade(client, "openunison", chartReq, ou.helmValues)

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

		chartReq, err := ou.locateChart(ou.orchestraChart, &client.ChartPathOptions, settings)

		if err != nil {
			return err
		}

		mergedValues := mergeMaps(chartReq.Values, ou.helmValues)

		//_, deployErr = client.Run(chartReq, mergedValues)
		_, deployErr = ou.runChartInstall(client, client.ReleaseName, chartReq, mergedValues, actionConfig)

	} else {
		// deploy orchestra, make sure that it deploys

		fmt.Println("Orchestra exists, upgrading")

		client := action.NewUpgrade(actionConfig)

		client.Namespace = ou.namespace

		chartReq, err := ou.locateChart(ou.orchestraChart, &client.ChartPathOptions, settings)

		if err != nil {
			return err
		}

		mergedValues := mergeMaps(chartReq.Values, ou.helmValues)

		//_, deployErr = client.Run("orchestra", chartReq, mergedValues)
		_, deployErr = ou.runChartUpgrade(client, "orchestra", chartReq, mergedValues)
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
		} else {
			return deployErr
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

	fmt.Printf("Waiting for a few seconds for the webhooks to settle to run")
	time.Sleep(10 * time.Second)

	fmt.Println("Deploying the orchestra-login-portal chart")

	if !orchestraLoginPortalDeployed {

		// deploy the orchestra-login-portal charts
		fmt.Println("orchestra-login-portal not deployed, installing")

		client := action.NewInstall(actionConfig)

		client.Namespace = ou.namespace
		client.ReleaseName = "orchestra-login-portal"

		chartReq, err := ou.locateChart(ou.orchestraLoginPortalChart, &client.ChartPathOptions, settings)

		if err != nil {
			return err
		}

		mergedValues := mergeMaps(chartReq.Values, ou.helmValues)

		//_, err = client.Run(chartReq, mergedValues)
		_, err = ou.runChartInstall(client, client.ReleaseName, chartReq, mergedValues, actionConfig)

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

		chartReq, err := ou.locateChart(ou.orchestraLoginPortalChart, &client.ChartPathOptions, settings)

		if err != nil {
			return err
		}

		mergedValues := mergeMaps(chartReq.Values, ou.helmValues)

		//_, err = client.Run("orchestra-login-portal", chartReq, mergedValues)
		_, err = ou.runChartUpgrade(client, "orchestra-login-portal", chartReq, mergedValues)

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

		openUnisonNamespace := &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name, Labels: ou.namespaceLabels}}

		_, err = ou.clientset.CoreV1().Namespaces().Create(context.TODO(), openUnisonNamespace, metav1.CreateOptions{})
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
