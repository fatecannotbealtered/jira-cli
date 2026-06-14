package cmd

import "testing"

// TestReference_EveryCommandHasRealSchemaAndExample guards against output_schema
// regressing to a stub: every leaf command must reference a schema label that
// exists in the Schemas map with a non-empty field list, and must carry at least
// one runnable example. This keeps `reference` a usable source of truth for agents.
func TestReference_EveryCommandHasRealSchemaAndExample(t *testing.T) {
	doc := buildReferenceDocument(rootCmd)
	if len(doc.Commands) == 0 {
		t.Fatal("reference enumerated zero commands")
	}
	if len(doc.Schemas) == 0 {
		t.Fatal("reference defined zero schemas")
	}
	for _, c := range doc.Commands {
		if c.OutputSchema == "" {
			t.Errorf("%s: empty output_schema", c.Path)
			continue
		}
		schema, ok := doc.Schemas[c.OutputSchema]
		if !ok {
			t.Errorf("%s: output_schema %q not defined in schemas map", c.Path, c.OutputSchema)
			continue
		}
		if len(schema.Fields) == 0 {
			t.Errorf("%s: schema %q has no fields (stub)", c.Path, c.OutputSchema)
		}
		if schema.Shape != "object" && schema.Shape != "array" {
			t.Errorf("%s: schema %q has invalid shape %q", c.Path, c.OutputSchema, schema.Shape)
		}
		if len(c.Examples) == 0 {
			t.Errorf("%s: no examples", c.Path)
		}
	}
}

// TestReference_SchemasHaveValidShape ensures every catalog entry is well-formed
// even if no command currently points at it.
func TestReference_SchemasHaveValidShape(t *testing.T) {
	for label, schema := range referenceSchemas() {
		if schema.Shape != "object" && schema.Shape != "array" {
			t.Errorf("schema %q: invalid shape %q", label, schema.Shape)
		}
		if len(schema.Fields) == 0 {
			t.Errorf("schema %q: empty field list", label)
		}
	}
}
