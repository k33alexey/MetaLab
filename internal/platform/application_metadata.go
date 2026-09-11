package platform

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/k33alexey/MetaLab/internal/appdb"
	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/publication"
	"github.com/k33alexey/MetaLab/internal/systemdb"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

type ApplicationObject struct {
	Kind  metadata.Kind `json:"kind"`
	Name  string        `json:"name"`
	Title string        `json:"title"`
}

type ApplicationForm struct {
	Descriptor metadata.FormDescriptor `json:"descriptor"`
	Custom     *metadata.ManagedForm   `json:"custom,omitempty"`
}

type ApplicationListPage struct {
	Rows       []ApplicationListRow `json:"rows"`
	NextCursor *uuid.UUID           `json:"nextCursor,omitempty"`
	PageSize   int                  `json:"pageSize"`
}

type ApplicationListRow struct {
	Reference uuid.UUID         `json:"reference"`
	Values    map[string]string `json:"values"`
}

// LoadApplicationObjects returns objects from the exact active publication.
func (runtime *Runtime) LoadApplicationObjects(ctx context.Context, token string, databaseID uuid.UUID, language string) ([]ApplicationObject, error) {
	if _, err := runtime.ResumePortalDatabase(ctx, token, databaseID); err != nil {
		return nil, err
	}
	snapshot, pool, err := runtime.loadPublishedMetadata(ctx, databaseID)
	if err != nil {
		return nil, err
	}
	defer pool.Close()
	catalog, err := snapshot.Catalog()
	if err != nil {
		return nil, err
	}
	result := make([]ApplicationObject, 0, len(catalog.Catalogs)+len(catalog.Documents))
	for _, item := range catalog.Catalogs {
		result = append(result, ApplicationObject{Kind: metadata.CatalogKind, Name: item.Name, Title: resolvedApplicationTitle(item.Title, item.Name, language, catalog)})
	}
	for _, item := range catalog.Documents {
		result = append(result, ApplicationObject{Kind: metadata.DocumentKind, Name: item.Name, Title: resolvedApplicationTitle(item.Title, item.Name, language, catalog)})
	}
	return result, nil
}

// LoadApplicationForm resolves generated or custom managed-form metadata.
func (runtime *Runtime) LoadApplicationForm(ctx context.Context, token string, databaseID uuid.UUID, objectKind metadata.Kind, name string, formKind metadata.FormKind, language string) (ApplicationForm, error) {
	if _, err := runtime.ResumePortalDatabase(ctx, token, databaseID); err != nil {
		return ApplicationForm{}, err
	}
	snapshot, pool, err := runtime.loadPublishedMetadata(ctx, databaseID)
	if err != nil {
		return ApplicationForm{}, err
	}
	defer pool.Close()
	catalog, err := snapshot.Catalog()
	if err != nil {
		return ApplicationForm{}, err
	}
	var descriptor metadata.FormDescriptor
	switch objectKind {
	case metadata.CatalogKind:
		descriptor, err = catalog.CatalogForm(name, formKind, language)
	case metadata.DocumentKind:
		descriptor, err = catalog.DocumentForm(name, formKind, language)
	default:
		err = fmt.Errorf("unsupported application object kind %q", objectKind)
	}
	if err != nil {
		return ApplicationForm{}, err
	}
	result := ApplicationForm{Descriptor: descriptor}
	if descriptor.SourceID != nil {
		form, ok := snapshot.Form(*descriptor.SourceID)
		if !ok {
			return ApplicationForm{}, fmt.Errorf("published managed form %s is missing", descriptor.SourceID)
		}
		result.Custom = &form
	}
	return result, nil
}

// LoadApplicationList returns one bounded metadata-aware page from PostgreSQL.
func (runtime *Runtime) LoadApplicationList(ctx context.Context, token string, databaseID uuid.UUID, objectKind metadata.Kind, name string, request metadata.DynamicListRequest) (ApplicationListPage, error) {
	if _, err := runtime.ResumePortalDatabase(ctx, token, databaseID); err != nil {
		return ApplicationListPage{}, err
	}
	snapshot, pool, err := runtime.loadPublishedMetadata(ctx, databaseID)
	if err != nil {
		return ApplicationListPage{}, err
	}
	defer pool.Close()
	catalog, err := snapshot.Catalog()
	if err != nil {
		return ApplicationListPage{}, err
	}
	result := ApplicationListPage{Rows: []ApplicationListRow{}, PageSize: request.Limit}
	switch objectKind {
	case metadata.CatalogKind:
		definition, ok := catalog.CatalogDefinition(name)
		if !ok {
			return ApplicationListPage{}, fmt.Errorf("unknown catalog %q", name)
		}
		repository, err := metadata.NewCatalogRepository(pool, catalog)
		if err != nil {
			return ApplicationListPage{}, err
		}
		page, err := repository.ListDynamic(ctx, name, request)
		if err != nil {
			return ApplicationListPage{}, err
		}
		result.PageSize, result.NextCursor = effectiveListPageSize(request.Limit, definition.List), page.NextCursor
		for _, record := range page.Records {
			values := map[string]string{
				"Ref": record.Reference.ObjectID.String(), "Code": record.Code, "Description": record.Description,
				"DeletionMark": fmt.Sprint(record.DeletionMark),
			}
			appendApplicationAttributes(values, definition.Attributes, record.Attributes)
			result.Rows = append(result.Rows, ApplicationListRow{Reference: record.Reference.ObjectID, Values: values})
		}
	case metadata.DocumentKind:
		definition, ok := catalog.DocumentDefinition(name)
		if !ok {
			return ApplicationListPage{}, fmt.Errorf("unknown document %q", name)
		}
		repository, err := metadata.NewDocumentRepository(pool, catalog)
		if err != nil {
			return ApplicationListPage{}, err
		}
		page, err := repository.ListDynamic(ctx, name, request)
		if err != nil {
			return ApplicationListPage{}, err
		}
		result.PageSize, result.NextCursor = effectiveListPageSize(request.Limit, definition.List), page.NextCursor
		for _, record := range page.Records {
			values := map[string]string{
				"Ref": record.Reference.ObjectID.String(), "Number": record.Number,
				"Date": record.Date.Format(time.RFC3339Nano), "Posted": fmt.Sprint(record.Posted),
				"DeletionMark": fmt.Sprint(record.DeletionMark),
			}
			appendApplicationAttributes(values, definition.Attributes, record.Attributes)
			result.Rows = append(result.Rows, ApplicationListRow{Reference: record.Reference.ObjectID, Values: values})
		}
	default:
		return ApplicationListPage{}, fmt.Errorf("unsupported application object kind %q", objectKind)
	}
	return result, nil
}

func appendApplicationAttributes(target map[string]string, attributes []metadata.Attribute, values map[uuid.UUID]metadata.Value) {
	for _, attribute := range attributes {
		if value, ok := values[attribute.ID]; ok {
			target[attribute.Name] = value.Data
		} else {
			target[attribute.Name] = ""
		}
	}
}

func effectiveListPageSize(requested int, settings metadata.ListSettings) int {
	if requested != 0 {
		return requested
	}
	if settings.PageSize != 0 {
		return settings.PageSize
	}
	return 20
}

func (runtime *Runtime) loadPublishedMetadata(ctx context.Context, databaseID uuid.UUID) (metadata.RuntimeSnapshot, *pgxpool.Pool, error) {
	pool, _, err := runtime.openApplicationPool(ctx, databaseID)
	if err != nil {
		return metadata.RuntimeSnapshot{}, nil, err
	}
	active, found, err := publication.Current(ctx, pool)
	if err != nil || !found {
		pool.Close()
		if err != nil {
			return metadata.RuntimeSnapshot{}, nil, err
		}
		return metadata.RuntimeSnapshot{}, nil, fmt.Errorf("application database has no active publication")
	}
	var manifest publication.Manifest
	if err := json.Unmarshal(active.Manifest, &manifest); err != nil {
		pool.Close()
		return metadata.RuntimeSnapshot{}, nil, fmt.Errorf("decode active publication metadata: %w", err)
	}
	if err := manifest.Runtime.Validate(); err != nil {
		pool.Close()
		return metadata.RuntimeSnapshot{}, nil, fmt.Errorf("validate active publication metadata: %w", err)
	}
	return manifest.Runtime, pool, nil
}

func (runtime *Runtime) openApplicationPool(ctx context.Context, id uuid.UUID) (*pgxpool.Pool, systemdb.RegisteredDatabase, error) {
	runtime.mu.RLock()
	database, secrets := runtime.database, runtime.secrets
	runtime.mu.RUnlock()
	if database == nil || secrets == nil {
		return nil, systemdb.RegisteredDatabase{}, fmt.Errorf("ML System PostgreSQL is not configured")
	}
	registered, err := database.Databases.Get(ctx, id)
	if err != nil {
		return nil, systemdb.RegisteredDatabase{}, err
	}
	if registered.State != systemdb.DatabaseRunning {
		return nil, systemdb.RegisteredDatabase{}, systemdb.ErrDatabaseNotRunning
	}
	password, err := secrets.Get(registered.Connection.SecretKey)
	if err != nil {
		return nil, systemdb.RegisteredDatabase{}, err
	}
	physicalID, err := appdb.EnsureIdentity(ctx, registered.Connection, password)
	if err != nil {
		return nil, systemdb.RegisteredDatabase{}, errors.New(safeOperationalError(err, password))
	}
	if physicalID != registered.PhysicalID {
		return nil, systemdb.RegisteredDatabase{}, fmt.Errorf("physical PostgreSQL database identity changed")
	}
	configuration, err := registered.Connection.PoolConfig(password)
	if err != nil {
		return nil, systemdb.RegisteredDatabase{}, errors.New(safeOperationalError(err, password))
	}
	configuration.MaxConns = 8
	pool, err := pgxpool.NewWithConfig(ctx, configuration)
	if err == nil {
		err = pool.Ping(ctx)
	}
	if err != nil {
		if pool != nil {
			pool.Close()
		}
		return nil, systemdb.RegisteredDatabase{}, errors.New(safeOperationalError(err, password))
	}
	return pool, registered, nil
}

func resolvedApplicationTitle(title metadata.LocalizedText, fallback, language string, catalog *metadata.Catalog) string {
	value := title.Resolve(strings.ToLower(strings.TrimSpace(language)), catalog.Project.Languages)
	if value == "" {
		return fallback
	}
	return value
}
