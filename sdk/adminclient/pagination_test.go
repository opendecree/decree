package adminclient

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestListSchemasPage_ReturnsPageAndToken(t *testing.T) {
	ms := &mockSchemaTransport{}
	client := New(WithSchemaTransport(ms))

	ms.listSchemasFn = func(_ context.Context, pageSize int32, pageToken string) (*ListSchemasResponse, error) {
		if pageSize != 25 {
			t.Errorf("got pageSize %d, want 25", pageSize)
		}
		if pageToken != "" {
			t.Errorf("got pageToken %q, want empty", pageToken)
		}
		return &ListSchemasResponse{
			Schemas:       []*Schema{{ID: "s1", Name: "a", Version: 1, CreatedAt: time.Now()}},
			NextPageToken: "cursor-2",
		}, nil
	}

	schemas, next, err := client.ListSchemasPage(context.Background(), 25, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(schemas) != 1 {
		t.Errorf("got len %d, want 1", len(schemas))
	}
	if next != "cursor-2" {
		t.Errorf("got next %q, want cursor-2", next)
	}
}

func TestListSchemasPage_LastPage(t *testing.T) {
	ms := &mockSchemaTransport{}
	client := New(WithSchemaTransport(ms))

	ms.listSchemasFn = func(_ context.Context, _ int32, _ string) (*ListSchemasResponse, error) {
		return &ListSchemasResponse{
			Schemas: []*Schema{{ID: "s2", Name: "b", Version: 1, CreatedAt: time.Now()}},
		}, nil
	}

	_, next, err := client.ListSchemasPage(context.Background(), 10, "cursor-2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if next != "" {
		t.Errorf("got next %q, want empty for last page", next)
	}
}

func TestListSchemasPage_ServiceNotConfigured(t *testing.T) {
	client := New()
	_, _, err := client.ListSchemasPage(context.Background(), 10, "")
	if !errors.Is(err, ErrServiceNotConfigured) {
		t.Errorf("got error %v, want ErrServiceNotConfigured", err)
	}
}

func TestListTenantsPage_ReturnsPageAndToken(t *testing.T) {
	ms := &mockSchemaTransport{}
	client := New(WithSchemaTransport(ms))

	ms.listTenantsFn = func(_ context.Context, schemaID *string, pageSize int32, pageToken string) (*ListTenantsResponse, error) {
		if schemaID == nil || *schemaID != "s1" {
			t.Errorf("got schemaID %v, want s1", schemaID)
		}
		if pageSize != 50 {
			t.Errorf("got pageSize %d, want 50", pageSize)
		}
		return &ListTenantsResponse{
			Tenants:       []*Tenant{{ID: "t1", Name: "acme"}},
			NextPageToken: "tok-2",
		}, nil
	}

	tenants, next, err := client.ListTenantsPage(context.Background(), "s1", 50, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tenants) != 1 {
		t.Errorf("got len %d, want 1", len(tenants))
	}
	if next != "tok-2" {
		t.Errorf("got next %q, want tok-2", next)
	}
}

func TestListTenantsPage_NoFilter(t *testing.T) {
	ms := &mockSchemaTransport{}
	client := New(WithSchemaTransport(ms))

	ms.listTenantsFn = func(_ context.Context, schemaID *string, _ int32, _ string) (*ListTenantsResponse, error) {
		if schemaID != nil {
			t.Errorf("expected nil schemaID, got %q", *schemaID)
		}
		return &ListTenantsResponse{}, nil
	}

	_, _, err := client.ListTenantsPage(context.Background(), "", 10, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListTenantsPage_ServiceNotConfigured(t *testing.T) {
	client := New()
	_, _, err := client.ListTenantsPage(context.Background(), "", 10, "")
	if !errors.Is(err, ErrServiceNotConfigured) {
		t.Errorf("got error %v, want ErrServiceNotConfigured", err)
	}
}

func TestListConfigVersionsPage_ReturnsPageAndToken(t *testing.T) {
	mc := &mockConfigTransport{}
	client := New(WithConfigTransport(mc))

	mc.listVersionsFn = func(_ context.Context, tenantID string, pageSize int32, pageToken string) (*ListVersionsResponse, error) {
		if tenantID != "t1" {
			t.Errorf("got tenantID %q, want t1", tenantID)
		}
		if pageSize != 20 {
			t.Errorf("got pageSize %d, want 20", pageSize)
		}
		return &ListVersionsResponse{
			Versions:      []*Version{{Version: 3, TenantID: "t1", CreatedAt: time.Now()}},
			NextPageToken: "v-tok",
		}, nil
	}

	versions, next, err := client.ListConfigVersionsPage(context.Background(), "t1", 20, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(versions) != 1 {
		t.Errorf("got len %d, want 1", len(versions))
	}
	if next != "v-tok" {
		t.Errorf("got next %q, want v-tok", next)
	}
}

func TestListConfigVersionsPage_ServiceNotConfigured(t *testing.T) {
	client := New()
	_, _, err := client.ListConfigVersionsPage(context.Background(), "t1", 10, "")
	if !errors.Is(err, ErrServiceNotConfigured) {
		t.Errorf("got error %v, want ErrServiceNotConfigured", err)
	}
}

func TestQueryWriteLogIter_ServiceNotConfigured(t *testing.T) {
	client := New()
	it := client.QueryWriteLogIter(context.Background())
	// Drain C before reading Err (C is already closed).
	for range it.C {
	}
	if err := <-it.Err; !errors.Is(err, ErrServiceNotConfigured) {
		t.Errorf("got error %v, want ErrServiceNotConfigured", err)
	}
}
