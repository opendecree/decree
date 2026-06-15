package docmodel_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/opendecree/decree/contrib/decree-docs/docmodel"
)

func ptrTo[T any](v T) *T { return &v }

func TestNew_StampsModelVersion(t *testing.T) {
	doc := docmodel.New(docmodel.Schema{Name: "payments"})
	if doc.DocModelVersion != docmodel.Version {
		t.Errorf("got DocModelVersion %d, want %d", doc.DocModelVersion, docmodel.Version)
	}
	if doc.Schema.Name != "payments" {
		t.Errorf("got schema name %q, want %q", doc.Schema.Name, "payments")
	}
}

// TestEncodeJSON_CanonicalForm pins the canonical serialization of a
// fully-populated document: lowerCamel keys, two-space indentation, and a
// trailing newline. A change to this expectation is a doc model change and
// must come with a docmodel.Version bump.
func TestEncodeJSON_CanonicalForm(t *testing.T) {
	doc := docmodel.New(docmodel.Schema{
		Name:               "payments",
		Description:        "Payment configuration",
		Version:            3,
		VersionDescription: "Added refund_window field",
		Info: &docmodel.Info{
			Title:   "Payments Configuration",
			Author:  "platform-team",
			Contact: &docmodel.Contact{Name: "Pat", Email: "pat@example.com", URL: "https://example.com/team"},
			Labels:  map[string]string{"team": "platform"},
		},
		Fields: []docmodel.Field{
			{
				Path:        "payments.webhook",
				Type:        "url",
				Title:       "Webhook URL",
				Description: "Webhook endpoint",
				Default:     "https://example.com/hook",
				Nullable:    true,
				Deprecated:  true,
				RedirectTo:  "payments.callback_url",
				Example:     "https://example.com/hook-example",
				Examples: map[string]docmodel.Example{
					"primary": {Value: "https://hooks.example.com/a", Summary: "Primary endpoint"},
				},
				ExternalDocs: &docmodel.ExternalDocs{
					Description: "Webhook guide",
					URL:         "https://docs.example.com/webhooks",
				},
				Tags:      []string{"billing"},
				Format:    "uri",
				ReadOnly:  true,
				WriteOnce: true,
				Sensitive: true,
				Constraints: &docmodel.Constraints{
					Minimum:          ptrTo(1.0),
					Maximum:          ptrTo(9.0),
					ExclusiveMinimum: ptrTo(0.5),
					ExclusiveMaximum: ptrTo(9.5),
					MinLength:        ptrTo(int32(2)),
					MaxLength:        ptrTo(int32(64)),
					Pattern:          "^https://",
					Enum:             []string{"https://example.com/hook"},
					JSONSchema:       `{"type":"string"}`,
					AllowedSchemes:   []string{"https", "sftp"},
				},
			},
		},
	})

	want := `{
  "docModelVersion": 1,
  "schema": {
    "name": "payments",
    "description": "Payment configuration",
    "version": 3,
    "versionDescription": "Added refund_window field",
    "info": {
      "title": "Payments Configuration",
      "author": "platform-team",
      "contact": {
        "name": "Pat",
        "email": "pat@example.com",
        "url": "https://example.com/team"
      },
      "labels": {
        "team": "platform"
      }
    },
    "fields": [
      {
        "path": "payments.webhook",
        "type": "url",
        "title": "Webhook URL",
        "description": "Webhook endpoint",
        "default": "https://example.com/hook",
        "nullable": true,
        "deprecated": true,
        "redirectTo": "payments.callback_url",
        "example": "https://example.com/hook-example",
        "examples": {
          "primary": {
            "value": "https://hooks.example.com/a",
            "summary": "Primary endpoint"
          }
        },
        "externalDocs": {
          "description": "Webhook guide",
          "url": "https://docs.example.com/webhooks"
        },
        "tags": [
          "billing"
        ],
        "format": "uri",
        "readOnly": true,
        "writeOnce": true,
        "sensitive": true,
        "constraints": {
          "minimum": 1,
          "maximum": 9,
          "exclusiveMinimum": 0.5,
          "exclusiveMaximum": 9.5,
          "minLength": 2,
          "maxLength": 64,
          "pattern": "^https://",
          "enum": [
            "https://example.com/hook"
          ],
          "jsonSchema": "{\"type\":\"string\"}",
          "allowedSchemes": [
            "https",
            "sftp"
          ]
        }
      }
    ]
  }
}
`
	var b strings.Builder
	if err := doc.EncodeJSON(&b); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := b.String(); got != want {
		t.Errorf("canonical JSON mismatch:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

// TestEncodeJSON_OmitsEmptyOptionals pins that optional zero values are
// dropped: a minimal schema serializes to only the required keys.
func TestEncodeJSON_OmitsEmptyOptionals(t *testing.T) {
	doc := docmodel.New(docmodel.Schema{
		Name:   "minimal",
		Fields: []docmodel.Field{{Path: "app.name", Type: "string"}},
	})

	want := `{
  "docModelVersion": 1,
  "schema": {
    "name": "minimal",
    "fields": [
      {
        "path": "app.name",
        "type": "string"
      }
    ]
  }
}
`
	var b strings.Builder
	if err := doc.EncodeJSON(&b); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := b.String(); got != want {
		t.Errorf("minimal JSON mismatch:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestEncodeJSON_DoesNotEscapeHTML(t *testing.T) {
	doc := docmodel.New(docmodel.Schema{
		Name:        "esc",
		Description: "a < b && c > d",
		Fields:      []docmodel.Field{{Path: "x", Type: "string"}},
	})
	var b strings.Builder
	if err := doc.EncodeJSON(&b); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(b.String(), "a < b && c > d") {
		t.Errorf("expected HTML characters to stay literal, got:\n%s", b.String())
	}
}

// failWriter rejects every write.
type failWriter struct{}

func (failWriter) Write([]byte) (int, error) { return 0, errors.New("disk full") }

func TestEncodeJSON_WriteError(t *testing.T) {
	doc := docmodel.New(docmodel.Schema{Name: "x", Fields: []docmodel.Field{{Path: "y", Type: "bool"}}})
	if err := doc.EncodeJSON(failWriter{}); err == nil {
		t.Error("expected write error, got nil")
	}
}
