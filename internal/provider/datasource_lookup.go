package provider

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
)

// configureDataSourceClient extracts the API client from provider data.
// Returns nil before the provider is configured, which the framework does on
// purpose during validation — that is not an error.
func configureDataSourceClient(req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) *client {
	if req.ProviderData == nil {
		return nil
	}
	c, ok := req.ProviderData.(*client)
	if !ok {
		resp.Diagnostics.AddError("unexpected provider data type", fmt.Sprintf("got %T", req.ProviderData))
		return nil
	}
	return c
}

// lookupByName scans a list of items and returns the first one whose "name"
// field matches. Returns nil when not found.
func lookupByName(items []map[string]any, name string) map[string]any {
	for _, m := range items {
		if strFromMap(m, "name") == name {
			return m
		}
	}
	return nil
}
