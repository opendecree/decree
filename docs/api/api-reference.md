# Protocol Documentation
<a name="top"></a>

## Table of Contents

- [centralconfig/v1/types.proto](#centralconfig_v1_types-proto)
    - [AuditEntry](#centralconfig-v1-AuditEntry)
    - [Config](#centralconfig-v1-Config)
    - [ConfigChange](#centralconfig-v1-ConfigChange)
    - [ConfigValue](#centralconfig-v1-ConfigValue)
    - [ConfigVersion](#centralconfig-v1-ConfigVersion)
    - [ExternalDocs](#centralconfig-v1-ExternalDocs)
    - [FieldConstraints](#centralconfig-v1-FieldConstraints)
    - [FieldExample](#centralconfig-v1-FieldExample)
    - [FieldLock](#centralconfig-v1-FieldLock)
    - [Schema](#centralconfig-v1-Schema)
    - [SchemaContact](#centralconfig-v1-SchemaContact)
    - [SchemaField](#centralconfig-v1-SchemaField)
    - [SchemaField.ExamplesEntry](#centralconfig-v1-SchemaField-ExamplesEntry)
    - [SchemaInfo](#centralconfig-v1-SchemaInfo)
    - [SchemaInfo.LabelsEntry](#centralconfig-v1-SchemaInfo-LabelsEntry)
    - [Tenant](#centralconfig-v1-Tenant)
    - [TypedValue](#centralconfig-v1-TypedValue)
    - [UsageStats](#centralconfig-v1-UsageStats)
  
    - [FieldType](#centralconfig-v1-FieldType)
  
- [centralconfig/v1/audit_service.proto](#centralconfig_v1_audit_service-proto)
    - [GetFieldUsageRequest](#centralconfig-v1-GetFieldUsageRequest)
    - [GetFieldUsageResponse](#centralconfig-v1-GetFieldUsageResponse)
    - [GetTenantUsageRequest](#centralconfig-v1-GetTenantUsageRequest)
    - [GetTenantUsageResponse](#centralconfig-v1-GetTenantUsageResponse)
    - [GetUnusedFieldsRequest](#centralconfig-v1-GetUnusedFieldsRequest)
    - [GetUnusedFieldsResponse](#centralconfig-v1-GetUnusedFieldsResponse)
    - [QueryWriteLogRequest](#centralconfig-v1-QueryWriteLogRequest)
    - [QueryWriteLogResponse](#centralconfig-v1-QueryWriteLogResponse)
  
    - [AuditService](#centralconfig-v1-AuditService)
  
- [centralconfig/v1/config_service.proto](#centralconfig_v1_config_service-proto)
    - [ExportConfigRequest](#centralconfig-v1-ExportConfigRequest)
    - [ExportConfigResponse](#centralconfig-v1-ExportConfigResponse)
    - [FieldUpdate](#centralconfig-v1-FieldUpdate)
    - [GetConfigRequest](#centralconfig-v1-GetConfigRequest)
    - [GetConfigResponse](#centralconfig-v1-GetConfigResponse)
    - [GetFieldRequest](#centralconfig-v1-GetFieldRequest)
    - [GetFieldResponse](#centralconfig-v1-GetFieldResponse)
    - [GetFieldsRequest](#centralconfig-v1-GetFieldsRequest)
    - [GetFieldsResponse](#centralconfig-v1-GetFieldsResponse)
    - [GetVersionRequest](#centralconfig-v1-GetVersionRequest)
    - [GetVersionResponse](#centralconfig-v1-GetVersionResponse)
    - [ImportConfigRequest](#centralconfig-v1-ImportConfigRequest)
    - [ImportConfigResponse](#centralconfig-v1-ImportConfigResponse)
    - [ListVersionsRequest](#centralconfig-v1-ListVersionsRequest)
    - [ListVersionsResponse](#centralconfig-v1-ListVersionsResponse)
    - [RollbackToVersionRequest](#centralconfig-v1-RollbackToVersionRequest)
    - [RollbackToVersionResponse](#centralconfig-v1-RollbackToVersionResponse)
    - [SetFieldRequest](#centralconfig-v1-SetFieldRequest)
    - [SetFieldResponse](#centralconfig-v1-SetFieldResponse)
    - [SetFieldsRequest](#centralconfig-v1-SetFieldsRequest)
    - [SetFieldsResponse](#centralconfig-v1-SetFieldsResponse)
    - [SubscribeRequest](#centralconfig-v1-SubscribeRequest)
    - [SubscribeResponse](#centralconfig-v1-SubscribeResponse)
  
    - [ImportMode](#centralconfig-v1-ImportMode)
  
    - [ConfigService](#centralconfig-v1-ConfigService)
  
- [centralconfig/v1/schema_service.proto](#centralconfig_v1_schema_service-proto)
    - [CreateSchemaRequest](#centralconfig-v1-CreateSchemaRequest)
    - [CreateSchemaResponse](#centralconfig-v1-CreateSchemaResponse)
    - [CreateTenantRequest](#centralconfig-v1-CreateTenantRequest)
    - [CreateTenantResponse](#centralconfig-v1-CreateTenantResponse)
    - [DeleteSchemaRequest](#centralconfig-v1-DeleteSchemaRequest)
    - [DeleteSchemaResponse](#centralconfig-v1-DeleteSchemaResponse)
    - [DeleteTenantRequest](#centralconfig-v1-DeleteTenantRequest)
    - [DeleteTenantResponse](#centralconfig-v1-DeleteTenantResponse)
    - [ExportSchemaRequest](#centralconfig-v1-ExportSchemaRequest)
    - [ExportSchemaResponse](#centralconfig-v1-ExportSchemaResponse)
    - [GetSchemaRequest](#centralconfig-v1-GetSchemaRequest)
    - [GetSchemaResponse](#centralconfig-v1-GetSchemaResponse)
    - [GetTenantRequest](#centralconfig-v1-GetTenantRequest)
    - [GetTenantResponse](#centralconfig-v1-GetTenantResponse)
    - [ImportSchemaRequest](#centralconfig-v1-ImportSchemaRequest)
    - [ImportSchemaResponse](#centralconfig-v1-ImportSchemaResponse)
    - [ListFieldLocksRequest](#centralconfig-v1-ListFieldLocksRequest)
    - [ListFieldLocksResponse](#centralconfig-v1-ListFieldLocksResponse)
    - [ListSchemasRequest](#centralconfig-v1-ListSchemasRequest)
    - [ListSchemasResponse](#centralconfig-v1-ListSchemasResponse)
    - [ListTenantsRequest](#centralconfig-v1-ListTenantsRequest)
    - [ListTenantsResponse](#centralconfig-v1-ListTenantsResponse)
    - [LockFieldRequest](#centralconfig-v1-LockFieldRequest)
    - [LockFieldResponse](#centralconfig-v1-LockFieldResponse)
    - [PublishSchemaRequest](#centralconfig-v1-PublishSchemaRequest)
    - [PublishSchemaResponse](#centralconfig-v1-PublishSchemaResponse)
    - [UnlockFieldRequest](#centralconfig-v1-UnlockFieldRequest)
    - [UnlockFieldResponse](#centralconfig-v1-UnlockFieldResponse)
    - [UpdateSchemaRequest](#centralconfig-v1-UpdateSchemaRequest)
    - [UpdateSchemaResponse](#centralconfig-v1-UpdateSchemaResponse)
    - [UpdateTenantRequest](#centralconfig-v1-UpdateTenantRequest)
    - [UpdateTenantResponse](#centralconfig-v1-UpdateTenantResponse)
  
    - [SchemaService](#centralconfig-v1-SchemaService)
  
- [centralconfig/v1/version_service.proto](#centralconfig_v1_version_service-proto)
    - [GetServerVersionRequest](#centralconfig-v1-GetServerVersionRequest)
    - [GetServerVersionResponse](#centralconfig-v1-GetServerVersionResponse)
  
    - [VersionService](#centralconfig-v1-VersionService)
  
- [Scalar Value Types](#scalar-value-types)



<a name="centralconfig_v1_types-proto"></a>
<p align="right"><a href="#top">Top</a></p>

## centralconfig/v1/types.proto



<a name="centralconfig-v1-AuditEntry"></a>

### AuditEntry
AuditEntry represents a write event recorded in the audit log.
Every config mutation (SetField, SetFields, RollbackToVersion) creates
one or more audit entries atomically with the config change.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [string](#string) |  | Server-assigned unique identifier (UUID). |
| tenant_id | [string](#string) |  | The tenant affected (UUID). |
| actor | [string](#string) |  | The actor who performed the action (from JWT subject). |
| action | [string](#string) |  | The action type (e.g. &#34;set_field&#34;, &#34;rollback&#34;). |
| field_path | [string](#string) | optional | The field that was changed. Present for set_field actions. |
| old_value | [string](#string) | optional | The previous value. Present for set_field actions. |
| new_value | [string](#string) | optional | The new value. Present for set_field actions. For rollback actions, contains the target version (e.g. &#34;v2&#34;). |
| config_version | [int32](#int32) | optional | The config version number created by this action. |
| created_at | [google.protobuf.Timestamp](#google-protobuf-Timestamp) |  | When the audit entry was created. |






<a name="centralconfig-v1-Config"></a>

### Config
Config represents the full resolved configuration for a tenant at a specific version.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | The tenant this config belongs to (UUID). |
| version | [int32](#int32) |  | The version number this config was resolved at. |
| values | [ConfigValue](#centralconfig-v1-ConfigValue) | repeated | All configuration values at this version. |






<a name="centralconfig-v1-ConfigChange"></a>

### ConfigChange
ConfigChange represents a real-time change event pushed to subscribers
via the Subscribe streaming RPC.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | The tenant whose config changed (UUID). |
| version | [int32](#int32) |  | The new config version number created by this change. |
| field_path | [string](#string) |  | The field that was changed. |
| old_value | [TypedValue](#centralconfig-v1-TypedValue) |  | The previous value. Absent if field was newly created or was null. |
| new_value | [TypedValue](#centralconfig-v1-TypedValue) |  | The new value. Absent if field was set to null. |
| changed_by | [string](#string) |  | The actor who made the change. |
| changed_at | [google.protobuf.Timestamp](#google-protobuf-Timestamp) |  | When the change occurred. |






<a name="centralconfig-v1-ConfigValue"></a>

### ConfigValue
ConfigValue represents a single configuration value at a point in time.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| field_path | [string](#string) |  | Dot-separated field path (e.g. &#34;payments.fee&#34;). |
| value | [TypedValue](#centralconfig-v1-TypedValue) |  | The typed value. Absent when the field value is null. |
| checksum | [string](#string) |  | Checksum of the value (xxHash). Used for optimistic concurrency control in SetField/SetFields via expected_checksum. |
| description | [string](#string) | optional | Human-readable description explaining this specific value. Only populated when include_description(s) is true in the request. |






<a name="centralconfig-v1-ConfigVersion"></a>

### ConfigVersion
ConfigVersion represents a point-in-time snapshot of configuration changes.
Each write operation (SetField, SetFields, RollbackToVersion) creates a new
config version. Versions store only the changed fields (delta storage) — the
full config at any version is the union of all deltas up to that version.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [string](#string) |  | Server-assigned unique identifier (UUID). |
| tenant_id | [string](#string) |  | The tenant this version belongs to (UUID). |
| version | [int32](#int32) |  | Version number (monotonically increasing, starting at 1). |
| description | [string](#string) |  | Description of what changed in this version. |
| created_by | [string](#string) |  | The actor who created this version (from JWT subject, or &#34;unknown&#34; if auth is disabled). |
| created_at | [google.protobuf.Timestamp](#google-protobuf-Timestamp) |  | When this version was created. |






<a name="centralconfig-v1-ExternalDocs"></a>

### ExternalDocs
ExternalDocs links to external documentation.
OAS: External Documentation Object


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| description | [string](#string) |  | Human-readable description of the external documentation. |
| url | [string](#string) |  | URL to the external documentation. |






<a name="centralconfig-v1-FieldConstraints"></a>

### FieldConstraints
FieldConstraints defines validation rules for a schema field.
Which constraints apply depends on the field&#39;s type — see FieldType docs.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| min | [double](#double) | optional | For integer/number/duration: minimum allowed value (inclusive, &gt;=). |
| max | [double](#double) | optional | For integer/number/duration: maximum allowed value (inclusive, &lt;=). |
| regex | [string](#string) | optional | Regular expression pattern the value must match. Applies to string-typed fields. Uses RE2 syntax. |
| enum_values | [string](#string) | repeated | Allowed values. If non-empty, the value must be one of these. Applies to any field type. |
| json_schema | [string](#string) | optional | JSON Schema document for structural validation of json-typed fields. Encoded as a JSON string. |
| exclusive_min | [double](#double) | optional | For integer/number/duration: exclusive minimum (strict, &gt;). |
| exclusive_max | [double](#double) | optional | For integer/number/duration: exclusive maximum (strict, &lt;). |
| min_length | [int32](#int32) | optional | For string: minimum allowed length. |
| max_length | [int32](#int32) | optional | For string: maximum allowed length. |






<a name="centralconfig-v1-FieldExample"></a>

### FieldExample
FieldExample represents a named example value for a schema field.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| value | [string](#string) |  | The example value, encoded as a string matching the field type. |
| summary | [string](#string) |  | Short description of what this example demonstrates. |






<a name="centralconfig-v1-FieldLock"></a>

### FieldLock
FieldLock prevents a configuration field from being modified by non-superadmin users.
Superadmins bypass all field locks.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | The tenant this lock applies to (UUID). |
| field_path | [string](#string) |  | The dot-separated field path that is locked. |
| locked_values | [string](#string) | repeated | For enum fields: the specific subset of values that are locked (not editable by admin). If empty, the entire field is locked. |






<a name="centralconfig-v1-Schema"></a>

### Schema
Schema represents a configuration schema template.
Schemas define the allowed fields and their types for tenant configurations.
Each schema is versioned — updates create new immutable versions.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [string](#string) |  | Server-assigned unique identifier (UUID). |
| name | [string](#string) |  | Machine identifier for this schema, used in API calls, URLs, and code. Must be a unique slug: lowercase alphanumeric and hyphens, 1-63 characters, matching [a-z0-9]([a-z0-9-]*[a-z0-9])?. See info.title for a display name. |
| description | [string](#string) |  | Human-readable description of the schema&#39;s purpose. |
| version | [int32](#int32) |  | Schema version number (monotonically increasing, starting at 1). |
| parent_version | [int32](#int32) | optional | The version this was derived from. Absent for the initial version (v1). |
| version_description | [string](#string) |  | Description of what changed in this version. |
| checksum | [string](#string) |  | Deterministic checksum of the field definitions (type, constraints, path). Used for change detection on import. |
| published | [bool](#bool) |  | Whether this version is published. Only published versions can be assigned to tenants. Published versions are immutable. |
| fields | [SchemaField](#centralconfig-v1-SchemaField) | repeated | The fields defined in this schema version. |
| created_at | [google.protobuf.Timestamp](#google-protobuf-Timestamp) |  | When this version was created. |
| info | [SchemaInfo](#centralconfig-v1-SchemaInfo) |  | Optional schema metadata: ownership, contact, labels. |






<a name="centralconfig-v1-SchemaContact"></a>

### SchemaContact
SchemaContact contains contact information for a schema owner.
OAS: Contact Object


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| name | [string](#string) |  | Contact name (person or team). |
| email | [string](#string) |  | Contact email address. |
| url | [string](#string) |  | Contact URL (e.g. team wiki page). |






<a name="centralconfig-v1-SchemaField"></a>

### SchemaField
SchemaField defines a single field within a configuration schema.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| path | [string](#string) |  | Dot-separated hierarchical path (e.g. &#34;payments.settlement.window&#34;). Must be unique within a schema version. |
| type | [FieldType](#centralconfig-v1-FieldType) |  | The value type for this field. Controls validation behavior. |
| constraints | [FieldConstraints](#centralconfig-v1-FieldConstraints) |  | Validation constraints. Optional — when absent, any value of the correct type is accepted. |
| nullable | [bool](#bool) |  | Whether this field accepts empty/null values. |
| deprecated | [bool](#bool) |  | Whether this field is deprecated. Deprecated fields are still readable but clients should migrate to redirect_to if set. |
| redirect_to | [string](#string) | optional | When deprecated, reads of this field can be redirected to this path. |
| default_value | [string](#string) | optional | Default value for this field, encoded as a string matching the field type. |
| description | [string](#string) | optional | Human-readable description of the field&#39;s purpose. |
| title | [string](#string) | optional | Human-friendly display name (e.g. &#34;Fee Rate&#34; for path &#34;payments.fee_rate&#34;). OAS: title |
| example | [string](#string) | optional | Single example value, encoded as a string matching the field type. OAS: example |
| examples | [SchemaField.ExamplesEntry](#centralconfig-v1-SchemaField-ExamplesEntry) | repeated | Named examples with optional summary. Key is example name. OAS: examples |
| external_docs | [ExternalDocs](#centralconfig-v1-ExternalDocs) |  | Link to external documentation for this field. OAS: externalDocs |
| tags | [string](#string) | repeated | Tags for grouping and categorization beyond the dot-prefix hierarchy. OAS: x-tags |
| format | [string](#string) | optional | Semantic format hint within the base type (e.g. &#34;email&#34;, &#34;semver&#34;, &#34;percentage&#34;). Informational — not enforced by validation. OAS: format |
| read_only | [bool](#bool) |  | Whether this field is system-managed and not user-editable. OAS: readOnly |
| write_once | [bool](#bool) |  | Whether this field can only be set once and becomes immutable after. |
| sensitive | [bool](#bool) |  | Whether this field&#39;s value should be masked in logs and UI. |






<a name="centralconfig-v1-SchemaField-ExamplesEntry"></a>

### SchemaField.ExamplesEntry



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| key | [string](#string) |  |  |
| value | [FieldExample](#centralconfig-v1-FieldExample) |  |  |






<a name="centralconfig-v1-SchemaInfo"></a>

### SchemaInfo
SchemaInfo contains optional metadata about a schema&#39;s ownership and context.
Optional organizational metadata for a schema.
Schema.description lives on Schema itself (not here) because it&#39;s a
first-class field used in every listing, while these are optional extras.
OAS: Info Object


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| title | [string](#string) |  | Human-friendly display name (e.g. &#34;Payment Configuration&#34;). Unlike Schema.name (a machine slug), this is for UI and documentation. OAS: info.title |
| author | [string](#string) |  | Schema owner identifier (person, team, or service). |
| contact | [SchemaContact](#centralconfig-v1-SchemaContact) |  | Contact information for the schema owner. OAS: info.contact |
| labels | [SchemaInfo.LabelsEntry](#centralconfig-v1-SchemaInfo-LabelsEntry) | repeated | Key-value labels for filtering and categorization. |






<a name="centralconfig-v1-SchemaInfo-LabelsEntry"></a>

### SchemaInfo.LabelsEntry



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| key | [string](#string) |  |  |
| value | [string](#string) |  |  |






<a name="centralconfig-v1-Tenant"></a>

### Tenant
Tenant represents an organization or entity that has its own configuration
based on an assigned schema version.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [string](#string) |  | Server-assigned unique identifier (UUID). |
| name | [string](#string) |  | Unique name for this tenant. Must be a valid slug: lowercase alphanumeric characters and hyphens, 1-63 characters, matching [a-z0-9]([a-z0-9-]*[a-z0-9])?. |
| schema_id | [string](#string) |  | The schema this tenant&#39;s configuration is based on (UUID). |
| schema_version | [int32](#int32) |  | The specific schema version assigned to this tenant. Must reference a published schema version. |
| created_at | [google.protobuf.Timestamp](#google-protobuf-Timestamp) |  | When the tenant was created. |
| updated_at | [google.protobuf.Timestamp](#google-protobuf-Timestamp) |  | When the tenant was last updated (name or schema version change). |






<a name="centralconfig-v1-TypedValue"></a>

### TypedValue
TypedValue holds a configuration value with its native type.
An unset oneof (no field present) represents a null value.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| integer_value | [int64](#int64) |  | Integer value. |
| number_value | [double](#double) |  | Floating-point number value. |
| string_value | [string](#string) |  | Free-form string value. |
| bool_value | [bool](#bool) |  | Boolean value. |
| time_value | [google.protobuf.Timestamp](#google-protobuf-Timestamp) |  | Timestamp value. |
| duration_value | [google.protobuf.Duration](#google-protobuf-Duration) |  | Duration value. |
| url_value | [string](#string) |  | URL value (must be a valid absolute URL). |
| json_value | [string](#string) |  | JSON value (must be valid JSON). |






<a name="centralconfig-v1-UsageStats"></a>

### UsageStats
UsageStats represents aggregated read usage statistics for a config field.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | The tenant (UUID). |
| field_path | [string](#string) |  | The field path. |
| read_count | [int64](#int64) |  | Total number of reads across the queried time range. |
| last_read_by | [string](#string) | optional | The last actor who read this field (if tracked). |
| last_read_at | [google.protobuf.Timestamp](#google-protobuf-Timestamp) | optional | When this field was last read (if tracked). |





 


<a name="centralconfig-v1-FieldType"></a>

### FieldType
FieldType enumerates the supported configuration value types.
Each type maps to a specific field in the TypedValue oneof.

| Name | Number | Description |
| ---- | ------ | ----------- |
| FIELD_TYPE_UNSPECIFIED | 0 |  |
| FIELD_TYPE_INT | 1 | Integer value. Encoded as a decimal string (e.g. &#34;42&#34;, &#34;-1&#34;). Supports minimum/maximum constraints on the numeric value. |
| FIELD_TYPE_STRING | 2 | Free-form string value. Supports minimum/maximum constraints on string length, pattern (regex), and enum constraints. |
| FIELD_TYPE_TIME | 3 | Timestamp value. Encoded as an RFC 3339 string (e.g. &#34;2025-01-15T09:30:00Z&#34;). |
| FIELD_TYPE_DURATION | 4 | Duration value. Encoded as a Go-style duration string (e.g. &#34;24h&#34;, &#34;30m&#34;, &#34;500ms&#34;). Supports minimum/maximum constraints on the duration in seconds. |
| FIELD_TYPE_URL | 5 | URL value. Must be a valid absolute URL. |
| FIELD_TYPE_JSON | 6 | JSON value. Stored as a JSON-encoded string. Supports json_schema constraint for structural validation. |
| FIELD_TYPE_NUMBER | 7 | Floating-point number value. Encoded as a decimal string (e.g. &#34;3.14&#34;, &#34;0.025&#34;). Supports minimum/maximum constraints on the numeric value. |
| FIELD_TYPE_BOOL | 8 | Boolean value. Encoded as &#34;true&#34; or &#34;false&#34;. |


 

 

 



<a name="centralconfig_v1_audit_service-proto"></a>
<p align="right"><a href="#top">Top</a></p>

## centralconfig/v1/audit_service.proto



<a name="centralconfig-v1-GetFieldUsageRequest"></a>

### GetFieldUsageRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | Tenant ID (UUID). |
| field_path | [string](#string) |  | Dot-separated field path to query. |
| start_time | [google.protobuf.Timestamp](#google-protobuf-Timestamp) | optional | Start of the time range. If omitted, includes all historical data. |
| end_time | [google.protobuf.Timestamp](#google-protobuf-Timestamp) | optional | End of the time range. If omitted, includes up to the current time. |






<a name="centralconfig-v1-GetFieldUsageResponse"></a>

### GetFieldUsageResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| stats | [UsageStats](#centralconfig-v1-UsageStats) |  | Aggregated usage statistics across the queried time range. |






<a name="centralconfig-v1-GetTenantUsageRequest"></a>

### GetTenantUsageRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | Tenant ID (UUID). |
| start_time | [google.protobuf.Timestamp](#google-protobuf-Timestamp) | optional | Start of the time range. If omitted, includes all historical data. |
| end_time | [google.protobuf.Timestamp](#google-protobuf-Timestamp) | optional | End of the time range. If omitted, includes up to the current time. |






<a name="centralconfig-v1-GetTenantUsageResponse"></a>

### GetTenantUsageResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| field_stats | [UsageStats](#centralconfig-v1-UsageStats) | repeated | Per-field usage statistics, ordered by field path. |






<a name="centralconfig-v1-GetUnusedFieldsRequest"></a>

### GetUnusedFieldsRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | Tenant ID (UUID). |
| since | [google.protobuf.Timestamp](#google-protobuf-Timestamp) |  | Fields not read since this time are considered unused. Required — there is no &#34;all time&#34; default to avoid expensive full scans. |






<a name="centralconfig-v1-GetUnusedFieldsResponse"></a>

### GetUnusedFieldsResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| field_paths | [string](#string) | repeated | Dot-separated paths of fields with no reads since the specified time. |






<a name="centralconfig-v1-QueryWriteLogRequest"></a>

### QueryWriteLogRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) | optional | Filter by tenant ID (UUID). If omitted, returns entries across all tenants. |
| actor | [string](#string) | optional | Filter by actor (JWT subject). If omitted, matches all actors. |
| field_path | [string](#string) | optional | Filter by field path. If omitted, matches all fields. |
| start_time | [google.protobuf.Timestamp](#google-protobuf-Timestamp) | optional | Filter entries created at or after this time. If omitted, no lower bound. |
| end_time | [google.protobuf.Timestamp](#google-protobuf-Timestamp) | optional | Filter entries created at or before this time. If omitted, no upper bound. |
| page_size | [int32](#int32) |  | Maximum number of results to return. Defaults to 50, max 100. |
| page_token | [string](#string) |  | Pagination token from a previous QueryWriteLogResponse. |






<a name="centralconfig-v1-QueryWriteLogResponse"></a>

### QueryWriteLogResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| entries | [AuditEntry](#centralconfig-v1-AuditEntry) | repeated | Audit entries matching the filters, ordered by created_at descending. |
| next_page_token | [string](#string) |  | Token for the next page. Empty if no more results. |





 

 

 


<a name="centralconfig-v1-AuditService"></a>

### AuditService
AuditService provides read-only access to the change history and usage
statistics for tenant configurations.

Write events are recorded automatically by the ConfigService whenever
configuration values are modified (atomically with the config change).
Usage statistics are tracked asynchronously via batched read counters.

| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| QueryWriteLog | [QueryWriteLogRequest](#centralconfig-v1-QueryWriteLogRequest) | [QueryWriteLogResponse](#centralconfig-v1-QueryWriteLogResponse) | QueryWriteLog searches the audit log for config change events. All filter parameters are optional — omit a filter to match all values. |
| GetFieldUsage | [GetFieldUsageRequest](#centralconfig-v1-GetFieldUsageRequest) | [GetFieldUsageResponse](#centralconfig-v1-GetFieldUsageResponse) | GetFieldUsage returns aggregated read statistics for a specific field. |
| GetTenantUsage | [GetTenantUsageRequest](#centralconfig-v1-GetTenantUsageRequest) | [GetTenantUsageResponse](#centralconfig-v1-GetTenantUsageResponse) | GetTenantUsage returns aggregated read statistics for all fields of a tenant. |
| GetUnusedFields | [GetUnusedFieldsRequest](#centralconfig-v1-GetUnusedFieldsRequest) | [GetUnusedFieldsResponse](#centralconfig-v1-GetUnusedFieldsResponse) | GetUnusedFields returns field paths that have not been read since the given time. Useful for identifying configuration fields that may be safe to deprecate. |

 



<a name="centralconfig_v1_config_service-proto"></a>
<p align="right"><a href="#top">Top</a></p>

## centralconfig/v1/config_service.proto



<a name="centralconfig-v1-ExportConfigRequest"></a>

### ExportConfigRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | Tenant ID (UUID). |
| version | [int32](#int32) | optional | Config version to export. If omitted, exports the latest version. |






<a name="centralconfig-v1-ExportConfigResponse"></a>

### ExportConfigResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| yaml_content | [bytes](#bytes) |  | YAML-encoded configuration values. |






<a name="centralconfig-v1-FieldUpdate"></a>

### FieldUpdate
FieldUpdate represents a single field change within a SetFields batch.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| field_path | [string](#string) |  | Dot-separated field path. |
| value | [TypedValue](#centralconfig-v1-TypedValue) |  | The typed value. Omit to set the field to null. |
| expected_checksum | [string](#string) | optional | Optimistic concurrency control checksum for this specific field. See SetFieldRequest.expected_checksum for details. |
| value_description | [string](#string) | optional | Value-level description for this specific field. |






<a name="centralconfig-v1-GetConfigRequest"></a>

### GetConfigRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | Tenant ID (UUID). |
| version | [int32](#int32) | optional | Config version to retrieve. If omitted, returns the latest version. |
| include_descriptions | [bool](#bool) |  | When true, includes value-level descriptions in the response. This bypasses the Redis cache and reads directly from the database. |






<a name="centralconfig-v1-GetConfigResponse"></a>

### GetConfigResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| config | [Config](#centralconfig-v1-Config) |  | The full resolved configuration at the requested version. |






<a name="centralconfig-v1-GetFieldRequest"></a>

### GetFieldRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | Tenant ID (UUID). |
| field_path | [string](#string) |  | Dot-separated field path (e.g. &#34;payments.fee&#34;). |
| version | [int32](#int32) | optional | Config version to read from. If omitted, reads from the latest version. |
| include_description | [bool](#bool) |  | When true, includes the value-level description. This bypasses the Redis cache and reads directly from the database. |






<a name="centralconfig-v1-GetFieldResponse"></a>

### GetFieldResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| value | [ConfigValue](#centralconfig-v1-ConfigValue) |  | The configuration value. Returns NOT_FOUND if the field has no value set. |






<a name="centralconfig-v1-GetFieldsRequest"></a>

### GetFieldsRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | Tenant ID (UUID). |
| field_paths | [string](#string) | repeated | Dot-separated field paths to retrieve. |
| version | [int32](#int32) | optional | Config version to read from. If omitted, reads from the latest version. |
| include_descriptions | [bool](#bool) |  | When true, includes value-level descriptions. This bypasses the Redis cache and reads directly from the database. |






<a name="centralconfig-v1-GetFieldsResponse"></a>

### GetFieldsResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| values | [ConfigValue](#centralconfig-v1-ConfigValue) | repeated | The requested values. Fields that don&#39;t exist are omitted (not an error). |






<a name="centralconfig-v1-GetVersionRequest"></a>

### GetVersionRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | Tenant ID (UUID). |
| version | [int32](#int32) |  | The version number to retrieve. |






<a name="centralconfig-v1-GetVersionResponse"></a>

### GetVersionResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| config_version | [ConfigVersion](#centralconfig-v1-ConfigVersion) |  |  |






<a name="centralconfig-v1-ImportConfigRequest"></a>

### ImportConfigRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | Tenant ID (UUID). |
| yaml_content | [bytes](#bytes) |  | YAML-encoded configuration values to import. |
| description | [string](#string) | optional | Description for the new config version created by the import. |
| mode | [ImportMode](#centralconfig-v1-ImportMode) |  | Import mode. Defaults to IMPORT_MODE_MERGE. |






<a name="centralconfig-v1-ImportConfigResponse"></a>

### ImportConfigResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| config_version | [ConfigVersion](#centralconfig-v1-ConfigVersion) |  | The config version created by the import. |






<a name="centralconfig-v1-ListVersionsRequest"></a>

### ListVersionsRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | Tenant ID (UUID). |
| page_size | [int32](#int32) |  | Maximum number of results to return. Defaults to 50, max 100. |
| page_token | [string](#string) |  | Pagination token from a previous ListVersionsResponse. |






<a name="centralconfig-v1-ListVersionsResponse"></a>

### ListVersionsResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| versions | [ConfigVersion](#centralconfig-v1-ConfigVersion) | repeated | Config versions, ordered by version number descending (newest first). |
| next_page_token | [string](#string) |  | Token for the next page. Empty if no more results. |






<a name="centralconfig-v1-RollbackToVersionRequest"></a>

### RollbackToVersionRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | Tenant ID (UUID). |
| version | [int32](#int32) |  | The target version to rollback to. The full config at this version is copied into a new version (the current version number &#43; 1). The target version itself is not modified. |
| description | [string](#string) | optional | Description for the new rollback version. Defaults to &#34;Rollback to version N&#34; if omitted. |






<a name="centralconfig-v1-RollbackToVersionResponse"></a>

### RollbackToVersionResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| config_version | [ConfigVersion](#centralconfig-v1-ConfigVersion) |  | The newly created config version containing the rolled-back values. |






<a name="centralconfig-v1-SetFieldRequest"></a>

### SetFieldRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | Tenant ID (UUID). |
| field_path | [string](#string) |  | Dot-separated field path (e.g. &#34;payments.fee&#34;). |
| value | [TypedValue](#centralconfig-v1-TypedValue) |  | The typed value. Omit to set the field to null. |
| expected_checksum | [string](#string) | optional | Optimistic concurrency control: the checksum from a previous GetField/GetConfig response. If provided and the field&#39;s current checksum doesn&#39;t match, the request fails with ABORTED. This prevents lost updates when multiple actors modify the same field concurrently. |
| description | [string](#string) | optional | Version-level description explaining why this change was made. |
| value_description | [string](#string) | optional | Value-level description explaining what this specific value means. Retrievable via include_description in read requests. |






<a name="centralconfig-v1-SetFieldResponse"></a>

### SetFieldResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| config_version | [ConfigVersion](#centralconfig-v1-ConfigVersion) |  | The newly created config version. |






<a name="centralconfig-v1-SetFieldsRequest"></a>

### SetFieldsRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | Tenant ID (UUID). |
| updates | [FieldUpdate](#centralconfig-v1-FieldUpdate) | repeated | Field updates to apply. All updates are applied atomically in a single config version. If any update fails validation (checksum, field lock), no changes are committed. |
| description | [string](#string) | optional | Version-level description explaining why these changes were made. |






<a name="centralconfig-v1-SetFieldsResponse"></a>

### SetFieldsResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| config_version | [ConfigVersion](#centralconfig-v1-ConfigVersion) |  | The newly created config version. |






<a name="centralconfig-v1-SubscribeRequest"></a>

### SubscribeRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | Tenant ID (UUID) to subscribe to. |
| field_paths | [string](#string) | repeated | Field paths to filter on. If empty, receives changes for all fields. Changes to fields not in this list are silently dropped. |






<a name="centralconfig-v1-SubscribeResponse"></a>

### SubscribeResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| change | [ConfigChange](#centralconfig-v1-ConfigChange) |  | A configuration change event. Delivered via Redis Pub/Sub internally. |





 


<a name="centralconfig-v1-ImportMode"></a>

### ImportMode
ImportMode controls how imported values interact with existing config.

| Name | Number | Description |
| ---- | ------ | ----------- |
| IMPORT_MODE_UNSPECIFIED | 0 | Unspecified defaults to merge behavior. |
| IMPORT_MODE_MERGE | 1 | Merge: update fields from YAML that differ, keep runtime overrides for fields not in the YAML. |
| IMPORT_MODE_REPLACE | 2 | Replace: full replace — all fields from YAML are set, fields not in YAML are not carried forward. Runtime overrides are wiped. |
| IMPORT_MODE_DEFAULTS | 3 | Defaults: only set fields that have no value yet. Fields that already have a value are skipped regardless of the YAML content. |


 

 


<a name="centralconfig-v1-ConfigService"></a>

### ConfigService
ConfigService manages configuration values, versions, and real-time subscriptions.

Configuration is stored per-tenant using delta versioning: each write creates a
new version containing only the changed fields. The full config at any version is
the union of all deltas up to that version (latest value per field wins).

Read operations are cached in Redis. Write operations are atomic: the config
version, values, and audit log entry are committed in a single database transaction.

Read operations.
Reads are served from Redis cache when possible. Setting include_descriptions
bypasses the cache and reads directly from the database.

| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| GetConfig | [GetConfigRequest](#centralconfig-v1-GetConfigRequest) | [GetConfigResponse](#centralconfig-v1-GetConfigResponse) | GetConfig returns the full resolved configuration for a tenant. |
| GetField | [GetFieldRequest](#centralconfig-v1-GetFieldRequest) | [GetFieldResponse](#centralconfig-v1-GetFieldResponse) | GetField returns a single configuration value by field path. |
| GetFields | [GetFieldsRequest](#centralconfig-v1-GetFieldsRequest) | [GetFieldsResponse](#centralconfig-v1-GetFieldsResponse) | GetFields returns multiple configuration values by field paths. Fields that don&#39;t exist are silently omitted from the response. |
| SetField | [SetFieldRequest](#centralconfig-v1-SetFieldRequest) | [SetFieldResponse](#centralconfig-v1-SetFieldResponse) | SetField sets a single configuration value, creating a new config version. |
| SetFields | [SetFieldsRequest](#centralconfig-v1-SetFieldsRequest) | [SetFieldsResponse](#centralconfig-v1-SetFieldsResponse) | SetFields sets multiple configuration values in a single config version. |
| ListVersions | [ListVersionsRequest](#centralconfig-v1-ListVersionsRequest) | [ListVersionsResponse](#centralconfig-v1-ListVersionsResponse) | ListVersions returns config version history for a tenant (newest first). |
| GetVersion | [GetVersionRequest](#centralconfig-v1-GetVersionRequest) | [GetVersionResponse](#centralconfig-v1-GetVersionResponse) | GetVersion retrieves metadata for a specific config version. |
| RollbackToVersion | [RollbackToVersionRequest](#centralconfig-v1-RollbackToVersionRequest) | [RollbackToVersionResponse](#centralconfig-v1-RollbackToVersionResponse) | RollbackToVersion creates a new version with the same values as the target version. This does not delete intermediate versions — it creates a new version that copies the target&#39;s values. |
| Subscribe | [SubscribeRequest](#centralconfig-v1-SubscribeRequest) | [SubscribeResponse](#centralconfig-v1-SubscribeResponse) stream | Subscribe opens a server-streaming connection that pushes ConfigChange events whenever the tenant&#39;s configuration is modified. The stream remains open until the client disconnects or the server shuts down. |
| ExportConfig | [ExportConfigRequest](#centralconfig-v1-ExportConfigRequest) | [ExportConfigResponse](#centralconfig-v1-ExportConfigResponse) | ExportConfig serializes a tenant&#39;s configuration to YAML. |
| ImportConfig | [ImportConfigRequest](#centralconfig-v1-ImportConfigRequest) | [ImportConfigResponse](#centralconfig-v1-ImportConfigResponse) | ImportConfig applies configuration values from YAML. |

 



<a name="centralconfig_v1_schema_service-proto"></a>
<p align="right"><a href="#top">Top</a></p>

## centralconfig/v1/schema_service.proto



<a name="centralconfig-v1-CreateSchemaRequest"></a>

### CreateSchemaRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| name | [string](#string) |  | Unique name for the schema. Must be a valid slug: lowercase alphanumeric and hyphens, 1-63 characters (e.g. &#34;payment-config&#34;, &#34;settlement-rules&#34;). |
| description | [string](#string) | optional | Human-readable description of the schema&#39;s purpose. |
| fields | [SchemaField](#centralconfig-v1-SchemaField) | repeated | Initial field definitions for version 1. At least one field is required. |






<a name="centralconfig-v1-CreateSchemaResponse"></a>

### CreateSchemaResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| schema | [Schema](#centralconfig-v1-Schema) |  | The created schema with version 1 (draft, unpublished). |






<a name="centralconfig-v1-CreateTenantRequest"></a>

### CreateTenantRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| name | [string](#string) |  | Unique name for the tenant. Must be a valid slug: lowercase alphanumeric and hyphens, 1-63 characters (e.g. &#34;acme-corp&#34;, &#34;tenant-42&#34;). |
| schema_id | [string](#string) |  | The schema to assign to this tenant (UUID). |
| schema_version | [int32](#int32) |  | The schema version to use. Must be a published version. |






<a name="centralconfig-v1-CreateTenantResponse"></a>

### CreateTenantResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant | [Tenant](#centralconfig-v1-Tenant) |  |  |






<a name="centralconfig-v1-DeleteSchemaRequest"></a>

### DeleteSchemaRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [string](#string) |  | Schema ID (UUID). Cascades to all versions, fields, and associated tenants. |






<a name="centralconfig-v1-DeleteSchemaResponse"></a>

### DeleteSchemaResponse







<a name="centralconfig-v1-DeleteTenantRequest"></a>

### DeleteTenantRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [string](#string) |  | Tenant ID (UUID). Cascades to all config versions, values, and field locks. |






<a name="centralconfig-v1-DeleteTenantResponse"></a>

### DeleteTenantResponse







<a name="centralconfig-v1-ExportSchemaRequest"></a>

### ExportSchemaRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [string](#string) |  | Schema ID (UUID). |
| version | [int32](#int32) | optional | Schema version to export. If omitted, exports the latest version. |






<a name="centralconfig-v1-ExportSchemaResponse"></a>

### ExportSchemaResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| yaml_content | [bytes](#bytes) |  | YAML-encoded schema (syntax v1). Includes schema name, description, version, and all field definitions with OAS-style constraint naming. Server-generated fields (id, checksum, published, created_at) are excluded. |






<a name="centralconfig-v1-GetSchemaRequest"></a>

### GetSchemaRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [string](#string) |  | Schema ID (UUID). |
| version | [int32](#int32) | optional | Schema version to retrieve. If omitted, returns the latest version. |






<a name="centralconfig-v1-GetSchemaResponse"></a>

### GetSchemaResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| schema | [Schema](#centralconfig-v1-Schema) |  |  |






<a name="centralconfig-v1-GetTenantRequest"></a>

### GetTenantRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [string](#string) |  | Tenant ID (UUID). |






<a name="centralconfig-v1-GetTenantResponse"></a>

### GetTenantResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant | [Tenant](#centralconfig-v1-Tenant) |  |  |






<a name="centralconfig-v1-ImportSchemaRequest"></a>

### ImportSchemaRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| yaml_content | [bytes](#bytes) |  | YAML-encoded schema (syntax v1). Must include `syntax`, `name`, and `fields`.

Import uses full-replace semantics: - If no schema with this name exists: creates a new schema with version 1. - If a schema exists and fields differ from latest: creates the next version. - If a schema exists and fields are identical: returns AlreadyExists error.

Imported versions are created as drafts (unpublished) unless auto_publish is true. The `version` field in the YAML is informational — the server assigns the next version number automatically. |
| auto_publish | [bool](#bool) |  | When true, the imported version is automatically published after creation. If the schema already exists and fields are identical (AlreadyExists), this has no effect. |






<a name="centralconfig-v1-ImportSchemaResponse"></a>

### ImportSchemaResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| schema | [Schema](#centralconfig-v1-Schema) |  | The created (or existing, on AlreadyExists) schema version. |






<a name="centralconfig-v1-ListFieldLocksRequest"></a>

### ListFieldLocksRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | Tenant ID (UUID). |






<a name="centralconfig-v1-ListFieldLocksResponse"></a>

### ListFieldLocksResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| locks | [FieldLock](#centralconfig-v1-FieldLock) | repeated | All active field locks for the tenant. |






<a name="centralconfig-v1-ListSchemasRequest"></a>

### ListSchemasRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| name_filter | [string](#string) | optional | Filter schemas by name prefix. Currently reserved for future use. |
| page_size | [int32](#int32) |  | Maximum number of results to return. Defaults to 50, max 100. |
| page_token | [string](#string) |  | Pagination token from a previous ListSchemasResponse. |






<a name="centralconfig-v1-ListSchemasResponse"></a>

### ListSchemasResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| schemas | [Schema](#centralconfig-v1-Schema) | repeated | Schemas with their latest version. |
| next_page_token | [string](#string) |  | Token for the next page. Empty if no more results. |






<a name="centralconfig-v1-ListTenantsRequest"></a>

### ListTenantsRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| schema_id | [string](#string) | optional | Filter by schema ID (UUID). If omitted, returns tenants across all schemas. |
| page_size | [int32](#int32) |  | Maximum number of results to return. Defaults to 50, max 100. |
| page_token | [string](#string) |  | Pagination token from a previous ListTenantsResponse. |






<a name="centralconfig-v1-ListTenantsResponse"></a>

### ListTenantsResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenants | [Tenant](#centralconfig-v1-Tenant) | repeated |  |
| next_page_token | [string](#string) |  | Token for the next page. Empty if no more results. |






<a name="centralconfig-v1-LockFieldRequest"></a>

### LockFieldRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | Tenant ID (UUID). |
| field_path | [string](#string) |  | Dot-separated field path to lock (e.g. &#34;payments.currency&#34;). |
| locked_values | [string](#string) | repeated | For enum fields: lock only these specific values. If empty, the entire field is locked regardless of value. |






<a name="centralconfig-v1-LockFieldResponse"></a>

### LockFieldResponse







<a name="centralconfig-v1-PublishSchemaRequest"></a>

### PublishSchemaRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [string](#string) |  | Schema ID (UUID). |
| version | [int32](#int32) |  | The version number to publish. Must be an existing draft version. Once published, the version is immutable. |






<a name="centralconfig-v1-PublishSchemaResponse"></a>

### PublishSchemaResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| schema | [Schema](#centralconfig-v1-Schema) |  | The published schema version. |






<a name="centralconfig-v1-UnlockFieldRequest"></a>

### UnlockFieldRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | Tenant ID (UUID). |
| field_path | [string](#string) |  | Dot-separated field path to unlock. |






<a name="centralconfig-v1-UnlockFieldResponse"></a>

### UnlockFieldResponse







<a name="centralconfig-v1-UpdateSchemaRequest"></a>

### UpdateSchemaRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [string](#string) |  | Schema ID (UUID). |
| version_description | [string](#string) | optional | Description of what changed in this version. |
| fields | [SchemaField](#centralconfig-v1-SchemaField) | repeated | Fields to add or modify. Existing fields not listed here are carried forward unchanged from the latest version. |
| remove_fields | [string](#string) | repeated | Dot-separated paths of fields to remove from the new version. |






<a name="centralconfig-v1-UpdateSchemaResponse"></a>

### UpdateSchemaResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| schema | [Schema](#centralconfig-v1-Schema) |  | The new schema version (draft, unpublished). |






<a name="centralconfig-v1-UpdateTenantRequest"></a>

### UpdateTenantRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [string](#string) |  | Tenant ID (UUID). |
| name | [string](#string) | optional | New name for the tenant. Must be a valid slug if provided. |
| schema_version | [int32](#int32) | optional | Upgrade to a new schema version. The new version must belong to the same schema and must be published. |






<a name="centralconfig-v1-UpdateTenantResponse"></a>

### UpdateTenantResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant | [Tenant](#centralconfig-v1-Tenant) |  |  |





 

 

 


<a name="centralconfig-v1-SchemaService"></a>

### SchemaService
SchemaService manages configuration schemas, tenants, and field-level locking.

Schemas define the allowed fields and their types for tenant configurations.
Each schema is versioned — updates create new immutable versions that must be
published before tenants can use them.

Schema lifecycle.

| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| CreateSchema | [CreateSchemaRequest](#centralconfig-v1-CreateSchemaRequest) | [CreateSchemaResponse](#centralconfig-v1-CreateSchemaResponse) | CreateSchema creates a new schema with an initial draft version (v1). |
| GetSchema | [GetSchemaRequest](#centralconfig-v1-GetSchemaRequest) | [GetSchemaResponse](#centralconfig-v1-GetSchemaResponse) | GetSchema retrieves a schema by ID, optionally at a specific version. |
| ListSchemas | [ListSchemasRequest](#centralconfig-v1-ListSchemasRequest) | [ListSchemasResponse](#centralconfig-v1-ListSchemasResponse) | ListSchemas returns all schemas, ordered by creation time (newest first). |
| UpdateSchema | [UpdateSchemaRequest](#centralconfig-v1-UpdateSchemaRequest) | [UpdateSchemaResponse](#centralconfig-v1-UpdateSchemaResponse) | UpdateSchema creates a new draft version by merging field changes with the latest version. |
| DeleteSchema | [DeleteSchemaRequest](#centralconfig-v1-DeleteSchemaRequest) | [DeleteSchemaResponse](#centralconfig-v1-DeleteSchemaResponse) | DeleteSchema permanently deletes a schema and all its versions. Cascades to tenants. |
| PublishSchema | [PublishSchemaRequest](#centralconfig-v1-PublishSchemaRequest) | [PublishSchemaResponse](#centralconfig-v1-PublishSchemaResponse) | PublishSchema marks a schema version as published and immutable. Only published versions can be assigned to tenants. |
| CreateTenant | [CreateTenantRequest](#centralconfig-v1-CreateTenantRequest) | [CreateTenantResponse](#centralconfig-v1-CreateTenantResponse) | CreateTenant creates a new tenant assigned to a published schema version. |
| GetTenant | [GetTenantRequest](#centralconfig-v1-GetTenantRequest) | [GetTenantResponse](#centralconfig-v1-GetTenantResponse) | GetTenant retrieves a tenant by ID. |
| ListTenants | [ListTenantsRequest](#centralconfig-v1-ListTenantsRequest) | [ListTenantsResponse](#centralconfig-v1-ListTenantsResponse) | ListTenants returns tenants, optionally filtered by schema. |
| UpdateTenant | [UpdateTenantRequest](#centralconfig-v1-UpdateTenantRequest) | [UpdateTenantResponse](#centralconfig-v1-UpdateTenantResponse) | UpdateTenant updates a tenant&#39;s name or upgrades its schema version. |
| DeleteTenant | [DeleteTenantRequest](#centralconfig-v1-DeleteTenantRequest) | [DeleteTenantResponse](#centralconfig-v1-DeleteTenantResponse) | DeleteTenant permanently deletes a tenant and all its configuration data. |
| LockField | [LockFieldRequest](#centralconfig-v1-LockFieldRequest) | [LockFieldResponse](#centralconfig-v1-LockFieldResponse) | LockField prevents a field from being modified. |
| UnlockField | [UnlockFieldRequest](#centralconfig-v1-UnlockFieldRequest) | [UnlockFieldResponse](#centralconfig-v1-UnlockFieldResponse) | UnlockField removes a field lock, allowing modifications again. |
| ListFieldLocks | [ListFieldLocksRequest](#centralconfig-v1-ListFieldLocksRequest) | [ListFieldLocksResponse](#centralconfig-v1-ListFieldLocksResponse) | ListFieldLocks returns all active field locks for a tenant. |
| ExportSchema | [ExportSchemaRequest](#centralconfig-v1-ExportSchemaRequest) | [ExportSchemaResponse](#centralconfig-v1-ExportSchemaResponse) | ExportSchema serializes a schema version to YAML. |
| ImportSchema | [ImportSchemaRequest](#centralconfig-v1-ImportSchemaRequest) | [ImportSchemaResponse](#centralconfig-v1-ImportSchemaResponse) | ImportSchema creates a schema (or new version) from YAML. Full-replace semantics: the YAML defines the complete field set. Returns AlreadyExists if the imported fields are identical to the latest version. |

 



<a name="centralconfig_v1_version_service-proto"></a>
<p align="right"><a href="#top">Top</a></p>

## centralconfig/v1/version_service.proto



<a name="centralconfig-v1-GetServerVersionRequest"></a>

### GetServerVersionRequest







<a name="centralconfig-v1-GetServerVersionResponse"></a>

### GetServerVersionResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| version | [string](#string) |  | Semantic version string (e.g. &#34;1.2.3&#34; or &#34;dev&#34;). |
| commit | [string](#string) |  | Git commit hash. |





 

 

 


<a name="centralconfig-v1-VersionService"></a>

### VersionService
VersionService provides server version information.

| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| GetServerVersion | [GetServerVersionRequest](#centralconfig-v1-GetServerVersionRequest) | [GetServerVersionResponse](#centralconfig-v1-GetServerVersionResponse) | GetServerVersion returns the server&#39;s build version and commit hash. |

 



## Scalar Value Types

| .proto Type | Notes | C++ | Java | Python | Go | C# | PHP | Ruby |
| ----------- | ----- | --- | ---- | ------ | -- | -- | --- | ---- |
| <a name="double" /> double |  | double | double | float | float64 | double | float | Float |
| <a name="float" /> float |  | float | float | float | float32 | float | float | Float |
| <a name="int32" /> int32 | Uses variable-length encoding. Inefficient for encoding negative numbers – if your field is likely to have negative values, use sint32 instead. | int32 | int | int | int32 | int | integer | Bignum or Fixnum (as required) |
| <a name="int64" /> int64 | Uses variable-length encoding. Inefficient for encoding negative numbers – if your field is likely to have negative values, use sint64 instead. | int64 | long | int/long | int64 | long | integer/string | Bignum |
| <a name="uint32" /> uint32 | Uses variable-length encoding. | uint32 | int | int/long | uint32 | uint | integer | Bignum or Fixnum (as required) |
| <a name="uint64" /> uint64 | Uses variable-length encoding. | uint64 | long | int/long | uint64 | ulong | integer/string | Bignum or Fixnum (as required) |
| <a name="sint32" /> sint32 | Uses variable-length encoding. Signed int value. These more efficiently encode negative numbers than regular int32s. | int32 | int | int | int32 | int | integer | Bignum or Fixnum (as required) |
| <a name="sint64" /> sint64 | Uses variable-length encoding. Signed int value. These more efficiently encode negative numbers than regular int64s. | int64 | long | int/long | int64 | long | integer/string | Bignum |
| <a name="fixed32" /> fixed32 | Always four bytes. More efficient than uint32 if values are often greater than 2^28. | uint32 | int | int | uint32 | uint | integer | Bignum or Fixnum (as required) |
| <a name="fixed64" /> fixed64 | Always eight bytes. More efficient than uint64 if values are often greater than 2^56. | uint64 | long | int/long | uint64 | ulong | integer/string | Bignum |
| <a name="sfixed32" /> sfixed32 | Always four bytes. | int32 | int | int | int32 | int | integer | Bignum or Fixnum (as required) |
| <a name="sfixed64" /> sfixed64 | Always eight bytes. | int64 | long | int/long | int64 | long | integer/string | Bignum |
| <a name="bool" /> bool |  | bool | boolean | boolean | bool | bool | boolean | TrueClass/FalseClass |
| <a name="string" /> string | A string must always contain UTF-8 encoded or 7-bit ASCII text. | string | String | str/unicode | string | string | string | String (UTF-8) |
| <a name="bytes" /> bytes | May contain any arbitrary sequence of bytes. | string | ByteString | str | []byte | ByteString | string | String (ASCII-8BIT) |

