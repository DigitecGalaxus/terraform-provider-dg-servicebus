package endpoint

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
)

func (r *endpointResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.AddError(
		"Import not supported yet",
		"Import is not supported for this resource",
	)
}
