package azure

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/azure-sdk-for-go/profiles/2017-03-09/resources/mgmt/resources"
	armStorage "github.com/Azure/azure-sdk-for-go/services/storage/mgmt/2016-05-01/storage"
	"github.com/Azure/azure-sdk-for-go/storage"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/adal"
	"github.com/hashicorp/go-azure-helpers/authentication"
)

type ArmClient struct {
	blobClient storage.BlobStorageClient

	// These Clients are only initialized if an Access Key isn't provided
	groupsClient          *resources.GroupsClient
	storageAccountsClient *armStorage.AccountsClient
}

func buildArmClient(ctx context.Context, config BackendConfig) (*ArmClient, error) {
	client := ArmClient{}
	env, err := authentication.DetermineEnvironment(config.Environment)
	if err != nil {
		return nil, err
	}

	// if we have an Access Key - we don't need the other clients
	if config.AccessKey != "" {
		storageClient, err := storage.NewClient(config.StorageAccountName, config.AccessKey, env.StorageEndpointSuffix, storage.DefaultAPIVersion, true)
		if err != nil {
			return nil, fmt.Errorf("Error creating storage client for storage account %q: %s", config.StorageAccountName, err)
		}
		client.blobClient = storageClient.GetBlobService()
		return &client, nil
	}

	builder := authentication.Builder{
		ClientID:       config.ClientID,
		ClientSecret:   config.ClientSecret,
		SubscriptionID: config.ClientSecret,
		TenantID:       config.TenantID,
		Environment:    config.Environment,

		// Feature Toggles
		SupportsClientSecretAuth: true,
		// TODO: support for Azure CLI / Client Certificate / MSI
	}
	armConfig, err := builder.Build()
	if err != nil {
		return nil, fmt.Errorf("Error building ARM Config: %+v", err)
	}

	oauthConfig, err := adal.NewOAuthConfig(env.ActiveDirectoryEndpoint, config.TenantID)
	if err != nil {
		return nil, err
	}

	auth, err := armConfig.GetAuthorizationToken(oauthConfig, env.ResourceManagerEndpoint)
	if err != nil {
		return nil, err
	}

	accountsClient := armStorage.NewAccountsClientWithBaseURI(env.ResourceManagerEndpoint, config.SubscriptionID)
	client.configureClient(&accountsClient.Client, auth)
	client.storageAccountsClient = &accountsClient

	groupsClient := resources.NewGroupsClientWithBaseURI(env.ResourceManagerEndpoint, config.SubscriptionID)
	client.configureClient(&groupsClient.Client, auth)
	client.groupsClient = &groupsClient

	accessKey, err := client.getAccessKey(ctx, config.ResourceGroupName, config.StorageAccountName)
	if err != nil {
		return nil, err
	}

	storageClient, err := storage.NewBasicClientOnSovereignCloud(config.StorageAccountName, *accessKey, *env)
	if err != nil {
		return nil, fmt.Errorf("Error creating storage client for storage account %q: %s", config.StorageAccountName, err)
	}
	client.blobClient = storageClient.GetBlobService()

	return &client, nil
}

func (c *ArmClient) configureClient(client *autorest.Client, auth autorest.Authorizer) {
	// TODO: fixme
	//setUserAgent(client)
	client.Authorizer = auth
	//client.RequestInspector = azure.WithClientID(clientRequestID())
	//client.Sender = azure.BuildSender()
	client.SkipResourceProviderRegistration = false
	client.PollingDuration = 60 * time.Minute
}

func (b *ArmClient) getAccessKey(ctx context.Context, resourceGroup, accountName string) (*string, error) {
	keys, err := b.storageAccountsClient.ListKeys(ctx, resourceGroup, accountName)
	if err != nil {
		return nil, fmt.Errorf("Error retrieving keys for Storage Account %q: %s", accountName, err)
	}

	if keys.Keys == nil {
		return nil, fmt.Errorf("Nil key returned for storage account %q", accountName)
	}

	accessKeys := *keys.Keys
	return accessKeys[0].Value, nil
}
