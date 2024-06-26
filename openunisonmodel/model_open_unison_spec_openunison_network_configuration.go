/*
 * OpenUnison CRD
 *
 * OpenUnison ScaleJS Register API
 *
 * API version: v6
 * Generated by: Swagger Codegen (https://github.com/swagger-api/swagger-codegen.git)
 */
package openunisonmodel

type OpenUnisonSpecOpenunisonNetworkConfiguration struct {
	ForceToLowerCase bool `json:"force_to_lower_case,omitempty"`
	OpenPort int32 `json:"open_port,omitempty"`
	OpenExternalPort int32 `json:"open_external_port,omitempty"`
	SecurePort int32 `json:"secure_port,omitempty"`
	SecureExternalPort int32 `json:"secure_external_port,omitempty"`
	LdapPort int32 `json:"ldap_port,omitempty"`
	LdapsPort int32 `json:"ldaps_port,omitempty"`
	LdapsKeyName string `json:"ldaps_key_name,omitempty"`
	ForceToSecure bool `json:"force_to_secure,omitempty"`
	ActivemqDir string `json:"activemq_dir,omitempty"`
	ClientAuth string `json:"client_auth,omitempty"`
	AllowedClientNames []string `json:"allowed_client_names,omitempty"`
	Ciphers []string `json:"ciphers,omitempty"`
	PathToDeployment string `json:"path_to_deployment,omitempty"`
	PathToEnvFile string `json:"path_to_env_file,omitempty"`
	SecureKeyAlias string `json:"secure_key_alias,omitempty"`
	AllowedTlsProtocols []string `json:"allowed_tls_protocols,omitempty"`
	QuartzDir string `json:"quartz_dir,omitempty"`
	ContextRoot string `json:"context_root,omitempty"`
	DisableHttp2 bool `json:"disable_http2,omitempty"`
	AllowUnEscapedChars string `json:"allow_un_escaped_chars,omitempty"`
	WelecomePages []string `json:"welecome_pages,omitempty"`
	ErrorPages []OpenUnisonSpecOpenunisonNetworkConfigurationErrorPages `json:"error_pages,omitempty"`
	RedirectToContextRoot bool `json:"redirect_to_context_root,omitempty"`
	QueueConfiguration *OpenUnisonSpecOpenunisonNetworkConfigurationQueueConfiguration `json:"queue_configuration,omitempty"`
}
