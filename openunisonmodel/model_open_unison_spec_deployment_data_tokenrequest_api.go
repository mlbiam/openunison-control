/*
 * OpenUnison CRD
 *
 * OpenUnison ScaleJS Register API
 *
 * API version: v6
 * Generated by: Swagger Codegen (https://github.com/swagger-api/swagger-codegen.git)
 */
package openunisonmodel

type OpenUnisonSpecDeploymentDataTokenrequestApi struct {
	Enabled bool `json:"enabled,omitempty"`
	Audience string `json:"audience,omitempty"`
	ExpirationSeconds int32 `json:"expirationSeconds,omitempty"`
}