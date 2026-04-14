//go:build e2e

package e2e

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func httpAddr() string {
	if addr := os.Getenv("HTTP_ADDR"); addr != "" {
		return addr
	}
	return "http://localhost:8080"
}

func httpGet(t *testing.T, path string) (int, map[string]any) {
	t.Helper()
	req, err := http.NewRequest("GET", httpAddr()+path, nil)
	require.NoError(t, err)
	req.Header.Set("x-subject", "e2e-test")
	req.Header.Set("x-role", "superadmin")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var result map[string]any
	_ = json.Unmarshal(body, &result)
	return resp.StatusCode, result
}

func httpPost(t *testing.T, path string, body any) (int, map[string]any) {
	t.Helper()
	data, err := json.Marshal(body)
	require.NoError(t, err)
	req, err := http.NewRequest("POST", httpAddr()+path, strings.NewReader(string(data)))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-subject", "e2e-test")
	req.Header.Set("x-role", "superadmin")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]any
	_ = json.Unmarshal(respBody, &result)
	return resp.StatusCode, result
}

func httpDelete(t *testing.T, path string) int {
	t.Helper()
	req, err := http.NewRequest("DELETE", httpAddr()+path, nil)
	require.NoError(t, err)
	req.Header.Set("x-subject", "e2e-test")
	req.Header.Set("x-role", "superadmin")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	return resp.StatusCode
}

// TestREST_Version verifies the version endpoint works over REST.
func TestREST_Version(t *testing.T) {
	status, body := httpGet(t, "/v1/version")
	assert.Equal(t, 200, status)
	assert.Contains(t, body, "version")
	assert.Contains(t, body, "commit")
}

// TestREST_SchemaLifecycle tests schema CRUD over REST.
func TestREST_SchemaLifecycle(t *testing.T) {
	// Create schema.
	createBody := map[string]any{
		"name": "rest-test-schema",
		"fields": []map[string]any{
			{"path": "rate", "type": 7, "nullable": true},
			{"path": "name", "type": 2},
		},
	}
	status, resp := httpPost(t, "/v1/schemas", createBody)
	require.Equal(t, 200, status, "create schema failed: %v", resp)

	schema := resp["schema"].(map[string]any)
	schemaID := schema["id"].(string)
	assert.Equal(t, "rest-test-schema", schema["name"])

	// Get schema.
	status, resp = httpGet(t, "/v1/schemas/"+schemaID)
	assert.Equal(t, 200, status)

	// List schemas.
	status, resp = httpGet(t, "/v1/schemas")
	assert.Equal(t, 200, status)
	schemas := resp["schemas"].([]any)
	assert.GreaterOrEqual(t, len(schemas), 1)

	// Publish schema.
	status, _ = httpPost(t, fmt.Sprintf("/v1/schemas/%s/publish", schemaID), map[string]any{"version": 1})
	assert.Equal(t, 200, status)

	// Create tenant.
	tenantBody := map[string]any{
		"name":           "rest-test-tenant",
		"schemaId":       schemaID,
		"schemaVersion":  1,
	}
	status, resp = httpPost(t, "/v1/tenants", tenantBody)
	require.Equal(t, 200, status, "create tenant failed: %v", resp)

	tenant := resp["tenant"].(map[string]any)
	tenantID := tenant["id"].(string)

	// Get tenant.
	status, resp = httpGet(t, "/v1/tenants/"+tenantID)
	assert.Equal(t, 200, status)

	// List tenants.
	status, resp = httpGet(t, "/v1/tenants")
	assert.Equal(t, 200, status)

	// Get config (empty).
	status, _ = httpGet(t, fmt.Sprintf("/v1/tenants/%s/config", tenantID))
	assert.Equal(t, 200, status)

	// List versions.
	status, _ = httpGet(t, fmt.Sprintf("/v1/tenants/%s/versions", tenantID))
	assert.Equal(t, 200, status)

	// Cleanup.
	httpDelete(t, "/v1/tenants/"+tenantID)
	httpDelete(t, "/v1/schemas/"+schemaID)
}

// TestREST_AuthHeaders verifies auth headers are forwarded from HTTP to gRPC.
func TestREST_AuthHeaders(t *testing.T) {
	// Request with x-subject should succeed.
	req, err := http.NewRequest("GET", httpAddr()+"/v1/version", nil)
	require.NoError(t, err)
	req.Header.Set("x-subject", "e2e-test")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, 200, resp.StatusCode)
}

// TestREST_Docs verifies the Swagger UI and OpenAPI spec are served.
func TestREST_Docs(t *testing.T) {
	t.Run("swagger UI page", func(t *testing.T) {
		resp, err := http.Get(httpAddr() + "/docs")
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, 200, resp.StatusCode)
		body, _ := io.ReadAll(resp.Body)
		assert.Contains(t, string(body), "swagger-ui")
	})

	t.Run("openapi spec", func(t *testing.T) {
		resp, err := http.Get(httpAddr() + "/docs/openapi.json")
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, 200, resp.StatusCode)
		assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))
		var spec map[string]any
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&spec))
		assert.Contains(t, spec, "swagger")
	})
}

// TestREST_NotFound verifies proper error codes for nonexistent resources.
func TestREST_NotFound(t *testing.T) {
	// Use a valid UUID format that doesn't exist.
	status, _ := httpGet(t, "/v1/schemas/00000000-0000-0000-0000-000000000000")
	// gRPC NOT_FOUND maps to HTTP 404 via grpc-gateway.
	assert.Equal(t, 404, status)
}
