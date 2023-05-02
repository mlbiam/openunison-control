# OpenUnisonSpec

## Properties
Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**Image** | **string** |  | [optional] [default to null]
**Replicas** | **int32** |  | [optional] [default to null]
**EnableActivemq** | **bool** |  | [optional] [default to null]
**ActivemqImage** | **string** |  | [optional] [default to null]
**DestSecret** | **string** |  | [optional] [default to null]
**SourceSecret** | **string** |  | [optional] [default to null]
**SecretData** | **[]string** |  | [optional] [default to null]
**MyvdConfigmap** | **string** |  | [optional] [default to null]
**Openshift** | [***OpenUnisonSpecOpenshift**](OpenUnison_spec_openshift.md) |  | [optional] [default to null]
**Hosts** | [**[]OpenUnisonSpecHosts**](OpenUnison_spec_hosts.md) |  | [optional] [default to null]
**DeploymentData** | [***OpenUnisonSpecDeploymentData**](OpenUnison_spec_deployment_data.md) |  | [optional] [default to null]
**NonSecretData** | [**[]OpenUnisonSpecAnnotations**](OpenUnison_spec_annotations.md) |  | [optional] [default to null]
**OpenunisonNetworkConfiguration** | [***OpenUnisonSpecOpenunisonNetworkConfiguration**](OpenUnison_spec_openunison_network_configuration.md) |  | [optional] [default to null]
**SamlRemoteIdp** | [**[]OpenUnisonSpecSamlRemoteIdp**](OpenUnison_spec_saml_remote_idp.md) |  | [optional] [default to null]
**RunSql** | **string** |  | [optional] [default to null]
**SqlCheckQuery** | **string** |  | [optional] [default to null]
**KeyStore** | [***OpenUnisonSpecKeyStore**](OpenUnison_spec_key_store.md) |  | [optional] [default to null]

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)

