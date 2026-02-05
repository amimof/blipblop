package cmdutil

import (
	"bytes"
	"html/template"
	"testing"
)

func TestTemplateColorSyntax(t *testing.T) {
	tests := []struct {
		name           string
		templateString string
		data           *ServiceState
		shouldError    bool
		expectedOutput string
	}{
		{
			name:           "correct fg syntax",
			templateString: `{{ fgYellow .Name }}`,
			data: &ServiceState{
				Name: "test-service",
				Metadata: map[string]any{
					"phase": "running",
					"node":  "node-1",
				},
			},
			shouldError: false,
		},
		{
			name:           "incorrect fg syntax with pipe to old function",
			templateString: `{{ .Name | fg.FgYellow }}`,
			data: &ServiceState{
				Name: "test-service",
			},
			shouldError: true, // This will error
		},
		{
			name:           "complete template with metadata",
			templateString: `{{ fgYellow .Name }} {{ .Metadata.phase }} {{ .Metadata.node }}`,
			data: &ServiceState{
				Name: "test-service",
				Metadata: map[string]any{
					"phase": "running",
					"node":  "node-1",
				},
			},
			shouldError: false,
		},
		{
			name:           "multiple colors",
			templateString: `{{ fgYellow .Name }} {{ fgCyan .Metadata.phase }} {{ fgGreen .Metadata.node }}`,
			data: &ServiceState{
				Name: "test-service",
				Metadata: map[string]any{
					"phase": "running",
					"node":  "node-1",
				},
			},
			shouldError: false,
		},
		{
			name:           "with background color",
			templateString: `{{ bgBlue .Name }}`,
			data: &ServiceState{
				Name: "test-service",
			},
			shouldError: false,
		},
		{
			name:           "with attribute",
			templateString: `{{ bold .Name }}`,
			data: &ServiceState{
				Name: "test-service",
			},
			shouldError: false,
		},
		{
			name:           "using pipe syntax",
			templateString: `{{ .Name | fgYellow }} {{ .Metadata.phase | fgCyan }}`,
			data: &ServiceState{
				Name: "test-service",
				Metadata: map[string]any{
					"phase": "running",
					"node":  "node-1",
				},
			},
			shouldError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl, err := template.New("test").Funcs(templateFuncs).Parse(tt.templateString)
			if err != nil {
				if !tt.shouldError {
					t.Errorf("unexpected error parsing template: %v", err)
				}
				return
			}

			var buf bytes.Buffer
			err = tmpl.Execute(&buf, tt.data)

			if tt.shouldError && err == nil {
				t.Error("expected error but got none")
			}

			if !tt.shouldError && err != nil {
				t.Errorf("unexpected error executing template: %v", err)
			}

			if !tt.shouldError && err == nil {
				t.Logf("Output: %s", buf.String())
			}
		})
	}
}
