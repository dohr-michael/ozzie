package skills

import (
	"context"
	"slices"
	"testing"
)

func TestSkillTool_Info_Simple(t *testing.T) {
	skill := &Skill{
		Name:        "greet",
		Description: "Greets the user",
		Type:        SkillTypeSimple,
		Instruction: "Say hello",
	}
	st := NewSkillTool(skill, RunnerConfig{})

	info, err := st.Info(context.Background())
	if err != nil {
		t.Fatalf("Info: %v", err)
	}
	if info.Name != "greet" {
		t.Fatalf("expected name %q, got %q", "greet", info.Name)
	}
	if info.Desc != "Greets the user" {
		t.Fatalf("expected desc %q, got %q", "Greets the user", info.Desc)
	}
	if info.ParamsOneOf == nil {
		t.Fatal("expected params to be set")
	}

	js, err := info.ParamsOneOf.ToJSONSchema()
	if err != nil {
		t.Fatalf("ToJSONSchema: %v", err)
	}
	if js.Properties == nil {
		t.Fatal("expected properties in JSON schema")
	}
	if _, ok := js.Properties.Get("request"); !ok {
		t.Fatal("expected 'request' property in schema")
	}
	if !slices.Contains(js.Required, "request") {
		t.Fatal("expected 'request' to be required for simple skills")
	}
}

func TestSkillTool_Info_Workflow(t *testing.T) {
	skill := &Skill{
		Name:        "deploy",
		Description: "Deploy pipeline",
		Type:        SkillTypeWorkflow,
		Vars: map[string]Var{
			"env":     {Description: "Target environment", Required: true},
			"version": {Description: "Version to deploy", Required: false},
		},
		Steps: []Step{
			{ID: "s1", Instruction: "do"},
		},
	}
	st := NewSkillTool(skill, RunnerConfig{})

	info, err := st.Info(context.Background())
	if err != nil {
		t.Fatalf("Info: %v", err)
	}

	js, err := info.ParamsOneOf.ToJSONSchema()
	if err != nil {
		t.Fatalf("ToJSONSchema: %v", err)
	}

	// Should have: env, version, request
	if js.Properties.Len() != 3 {
		t.Fatalf("expected 3 properties, got %d", js.Properties.Len())
	}
	if _, ok := js.Properties.Get("env"); !ok {
		t.Fatal("expected 'env' property")
	}
	if _, ok := js.Properties.Get("version"); !ok {
		t.Fatal("expected 'version' property")
	}
	if _, ok := js.Properties.Get("request"); !ok {
		t.Fatal("expected 'request' property")
	}
	if !slices.Contains(js.Required, "env") {
		t.Fatal("expected 'env' to be required")
	}
	if slices.Contains(js.Required, "request") {
		t.Fatal("expected 'request' to be optional for workflow skills")
	}
}

func TestSkillToManifest_Simple(t *testing.T) {
	skill := &Skill{
		Name:        "greet",
		Description: "Greets the user",
		Type:        SkillTypeSimple,
		Instruction: "Say hello",
	}

	m := SkillToManifest(skill)
	if m.Name != "greet" {
		t.Fatalf("expected name %q, got %q", "greet", m.Name)
	}
	if m.Level != "tool" {
		t.Fatalf("expected level %q, got %q", "tool", m.Level)
	}
	if m.Provider != "skill" {
		t.Fatalf("expected provider %q, got %q", "skill", m.Provider)
	}
	if len(m.Tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(m.Tools))
	}
	tool := m.Tools[0]
	if tool.Name != "greet" {
		t.Fatalf("expected tool name %q, got %q", "greet", tool.Name)
	}
	reqParam, ok := tool.Parameters["request"]
	if !ok {
		t.Fatal("expected 'request' param in manifest tool")
	}
	if !reqParam.Required {
		t.Fatal("expected 'request' param to be required")
	}
}

func TestSkillToManifest_Workflow(t *testing.T) {
	skill := &Skill{
		Name:        "deploy",
		Description: "Deploy pipeline",
		Type:        SkillTypeWorkflow,
		Vars: map[string]Var{
			"env": {Description: "Target environment", Required: true},
		},
		Steps: []Step{
			{ID: "s1", Instruction: "do"},
		},
	}

	m := SkillToManifest(skill)
	tool := m.Tools[0]

	// Should have: env + request
	if len(tool.Parameters) != 2 {
		t.Fatalf("expected 2 params, got %d", len(tool.Parameters))
	}
	envParam, ok := tool.Parameters["env"]
	if !ok {
		t.Fatal("expected 'env' param in manifest tool")
	}
	if !envParam.Required {
		t.Fatal("expected 'env' param to be required")
	}
	reqParam, ok := tool.Parameters["request"]
	if !ok {
		t.Fatal("expected 'request' param in manifest tool")
	}
	if reqParam.Required {
		t.Fatal("expected 'request' param to be optional for workflow")
	}
}
