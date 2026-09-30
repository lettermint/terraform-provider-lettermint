package contract

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestAnalyticsAndForwardingHaveExplicitCoverage(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", CoveragePath))
	if err != nil {
		t.Fatal(err)
	}
	var coverage Coverage
	if err := json.Unmarshal(data, &coverage); err != nil {
		t.Fatal(err)
	}
	if len(coverage.Operations) != 59 {
		t.Fatalf("got %d operations", len(coverage.Operations))
	}
	statuses := map[string]string{}
	for _, op := range coverage.Operations {
		statuses[op.Key] = op.Status
	}
	for _, operation := range []string{"v1.analytics", "getReportForwarding", "updateReportForwarding", "deleteReportForwarding", "verifyReportForwarding", "resendReportForwardingCode"} {
		if statuses["team#"+operation] != "intentionally-excluded" {
			t.Errorf("missing provider scope decision for %s", operation)
		}
	}
	fields := map[string]string{}
	for _, field := range coverage.Fields {
		fields[field.Key] = field.Status
	}
	for _, field := range []string{"ProjectListData/properties/delivery_mode", "StoreProjectData/properties/redact_email_content", "StoreRouteData/properties/settings", "UpdateRouteData/properties/inbound_domain", "WebhookListData/properties/delivery_mode_filter"} {
		if fields["team#/components/schemas/"+field] != "sdk-blocked" {
			t.Errorf("missing published SDK limit for %s", field)
		}
	}
	if fields["team#/components/schemas/ProjectCreatedData/properties/api_token"] != "implemented" {
		t.Error("project creation token must remain covered")
	}
}
