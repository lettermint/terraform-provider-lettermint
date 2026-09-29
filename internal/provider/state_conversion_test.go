package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/lettermint/lettermint-go/v2"
)

func TestCreateOnlySecretsStayInState(t *testing.T) {
	t.Parallel()

	project := projectModelFromAPI(lettermint.ProjectData{ID: "project-1"}, projectResourceModel{
		APIToken: types.StringValue("project-secret"),
	})
	if project.APIToken.ValueString() != "project-secret" {
		t.Fatalf("project API token = %q", project.APIToken.ValueString())
	}

	webhook := webhookModelFromAPI(context.Background(), lettermint.WebhookData{ID: "webhook-1"}, webhookResourceModel{
		Secret: types.StringValue("webhook-secret"),
	}, &diag.Diagnostics{})
	if webhook.Secret.ValueString() != "webhook-secret" {
		t.Fatalf("webhook secret = %q", webhook.Secret.ValueString())
	}
}
