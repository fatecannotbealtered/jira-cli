package cmd

import (
	"testing"

	"github.com/spf13/pflag"
)

func TestFormatFlagDefault(t *testing.T) {
	if got := formatFlagDefault(""); got != "-" {
		t.Fatalf("empty = %q", got)
	}
	if got := formatFlagDefault("[]"); got != "-" {
		t.Fatalf("slice empty = %q", got)
	}
	if got := formatFlagDefault("ok"); got != "ok" {
		t.Fatalf("value = %q", got)
	}
}

func TestCollectReferenceFlags_AllBranches(t *testing.T) {
	local := pflag.NewFlagSet("local", pflag.PanicOnError)
	persistent := pflag.NewFlagSet("persistent", pflag.PanicOnError)

	local.String("visible", "loc", "visible local")
	local.String("hidden-local", "x", "hidden local")
	local.Lookup("hidden-local").Hidden = true
	local.String("dup", "local-val", "dup local")

	persistent.String("persist", "", "persistent empty default")
	persistent.String("hidden-persist", "h", "hidden persistent")
	persistent.Lookup("hidden-persist").Hidden = true
	persistent.String("dup", "persist-val", "dup persistent")
	persistent.StringSlice("tags", nil, "tags")
	persistent.Lookup("tags").DefValue = "[]"

	flags := collectReferenceFlags(local, persistent, true)
	names := map[string]referenceFlag{}
	for _, f := range flags {
		names[f.Name] = f
	}
	if _, ok := names["visible"]; !ok {
		t.Fatal("missing visible")
	}
	if _, ok := names["hidden-local"]; ok {
		t.Fatal("hidden local should be omitted")
	}
	if _, ok := names["hidden-persist"]; ok {
		t.Fatal("hidden persistent should be omitted")
	}
	if _, ok := names["persist"]; !ok {
		t.Fatal("missing persist")
	}
	if names["persist"].Default != "-" {
		t.Fatalf("persist default = %q", names["persist"].Default)
	}
	if names["tags"].Default != "-" {
		t.Fatalf("tags default = %q", names["tags"].Default)
	}
	if _, ok := names["dup"]; !ok || names["dup"].Default != "local-val" {
		t.Fatalf("dup should prefer local once, got %+v", names["dup"])
	}

	noInherit := collectReferenceFlags(local, persistent, false)
	for _, f := range noInherit {
		if f.Name == "persist" {
			t.Fatal("persistent flags should not be collected when inheritPersistent is false")
		}
	}
}

func TestBuildReferenceDocumentReleaseReadiness(t *testing.T) {
	doc := buildReferenceDocument(rootCmd)
	if doc.ReleaseReadiness.Level != "stable" {
		t.Fatalf("release level = %q, want stable", doc.ReleaseReadiness.Level)
	}
	if !doc.ReleaseReadiness.FCCRequired || !doc.ReleaseReadiness.MockUpstreamRequired {
		t.Fatalf("release readiness should require FCC and mock upstream tests: %+v", doc.ReleaseReadiness)
	}
	if !doc.ReleaseReadiness.LiveSmokeRequiredForStable || doc.ReleaseReadiness.LiveSmokeStatus != "verified" {
		t.Fatalf("stable should have verified live smoke evidence: %+v", doc.ReleaseReadiness)
	}
}
