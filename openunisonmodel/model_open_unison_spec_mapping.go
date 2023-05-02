/*
 * OpenUnison CRD
 *
 * OpenUnison ScaleJS Register API
 *
 * API version: v6
 * Generated by: Swagger Codegen (https://github.com/swagger-api/swagger-codegen.git)
 */
package openunisonmodel

type OpenUnisonSpecMapping struct {
	EntityId string `json:"entity_id,omitempty"`
	PostUrl string `json:"post_url,omitempty"`
	RedirectUrl string `json:"redirect_url,omitempty"`
	LogoutUrl string `json:"logout_url,omitempty"`
	SigningCertAlias string `json:"signing_cert_alias,omitempty"`
	EncryptionCertAlias string `json:"encryption_cert_alias,omitempty"`
}
