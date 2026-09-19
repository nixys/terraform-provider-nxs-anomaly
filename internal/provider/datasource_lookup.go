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

// lookupByName scans a list of items and returns the one whose "name" field
// matches, plus how many matched.
//
// The count is the point. Names are not unique in nxs-anomaly, and returning
// whichever match came first meant a configuration could silently point at a
// different object than the author meant — and at a different one again after
// somebody renamed something. Callers turn a count above one into an error that
// says to use the id.
func lookupByName(items []map[string]any, name string) (map[string]any, int) {
	var found map[string]any
	matches := 0
	for _, m := range items {
		if strFromMap(m, "name") == name {
			matches++
			if found == nil {
				found = m
			}
		}
	}
	return found, matches
}

// ambiguousNameError is the message every data source gives for a name that
// matches more than one object, so the answer is the same wherever it is hit.
func ambiguousNameError(kind, name string, matches int) (string, string) {
	return fmt.Sprintf("ambiguous %s name", kind),
		fmt.Sprintf("%d %ss are named %q; look the object up by id instead", matches, kind, name)
}
