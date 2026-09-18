package platform

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/k33alexey/MetaLab/internal/appdb"
	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/publication"
	"github.com/k33alexey/MetaLab/internal/systemdb"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

type ApplicationObject struct {
	Kind  metadata.Kind `json:"kind"`
	Name  string        `json:"name"`
	Title string        `json:"title"`
	// Operations lists what this user may actually do with the object. ML App
	// needs it to offer the standard commands - "открыть список" is implied by
	// the object being here at all, "создать" is not - and offering a command
	// the caller cannot perform would be an invitation to an error message.
	Operations []metadata.PermissionOperation `json:"operations"`
}

type ApplicationForm struct {
	Descriptor metadata.FormDescriptor `json:"descriptor"`
	Custom     *metadata.ManagedForm   `json:"custom,omitempty"`
	// Language is the chain this form's titles were resolved through, so the
	// component that renders the custom part resolves the rest the same way
	// instead of guessing a language of its own.
	Language metadata.TitleLanguage `json:"language"`
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

// ApplicationObjects is what ML App starts from: the objects this user may
// reach and the language their titles were resolved in, which is also the
// language the interface should present itself in.
type ApplicationObjects struct {
	Objects  []ApplicationObject    `json:"objects"`
	Language metadata.TitleLanguage `json:"language"`
}

// LoadApplicationObjects returns objects from the exact active publication.
// LoadApplicationObjects lists only what the caller may actually read: an object
// the user has no grant on is absent from the menu rather than present and
// failing when opened.
func (runtime *Runtime) LoadApplicationObjects(ctx context.Context, token string, databaseID uuid.UUID, preferences []string) (ApplicationObjects, error) {
	session, err := runtime.ResumePortalDatabase(ctx, token, databaseID)
	if err != nil {
		return ApplicationObjects{}, err
	}
	snapshot, pool, err := runtime.loadPublishedMetadata(ctx, databaseID)
	if err != nil {
		return ApplicationObjects{}, err
	}
	defer pool.Close()
	catalog, err := snapshot.Catalog()
	if err != nil {
		return ApplicationObjects{}, err
	}
	permissions, err := runtime.applicationPermissions(ctx, databaseID, session.UserID, catalog)
	if err != nil {
		return ApplicationObjects{}, err
	}
	language := ApplicationLanguage(catalog.Project, preferences)
	result := make([]ApplicationObject, 0, len(catalog.Catalogs)+len(catalog.Documents))
	for _, item := range catalog.Catalogs {
		if !permissions.AllowsObject(item.ID, metadata.PermissionRead) {
			continue
		}
		result = append(result, ApplicationObject{Kind: metadata.CatalogKind, Name: item.Name,
			Title: resolvedApplicationTitle(item.Title, item.Name, language), Operations: allowedOperations(permissions, item.ID)})
	}
	for _, item := range catalog.Documents {
		if !permissions.AllowsObject(item.ID, metadata.PermissionRead) {
			continue
		}
		result = append(result, ApplicationObject{Kind: metadata.DocumentKind, Name: item.Name,
			Title: resolvedApplicationTitle(item.Title, item.Name, language), Operations: allowedOperations(permissions, item.ID)})
	}
	return ApplicationObjects{Objects: result, Language: language}, nil
}

// applicationObjectID resolves the metadata identity a read permission is keyed
// by. An unknown name is reported as unknown, never as denied, since the name
// came from the developer's own metadata, not from the caller's grants.
func applicationObjectID(catalog *metadata.Catalog, objectKind metadata.Kind, name string) (uuid.UUID, error) {
	switch objectKind {
	case metadata.CatalogKind:
		definition, ok := catalog.CatalogDefinition(name)
		if !ok {
			return uuid.UUID{}, fmt.Errorf("unknown catalog %q", name)
		}
		return definition.ID, nil
	case metadata.DocumentKind:
		definition, ok := catalog.DocumentDefinition(name)
		if !ok {
			return uuid.UUID{}, fmt.Errorf("unknown document %q", name)
		}
		return definition.ID, nil
	default:
		return uuid.UUID{}, fmt.Errorf("unsupported application object kind %q", objectKind)
	}
}

// requireApplicationRead resolves the caller's policy and refuses the whole
// operation when they may not read the object at all.
//
// snapshot and pool are what a restriction needs when it compares a field with
// a session parameter the PROJECT computes: answering that requires running the
// session module, and the runtime for it is built from them - but only if such
// a restriction is actually reached, never for an ordinary read.
func (runtime *Runtime) requireApplicationRead(ctx context.Context, databaseID, userID uuid.UUID, snapshot metadata.RuntimeSnapshot, pool *pgxpool.Pool, catalog *metadata.Catalog, objectKind metadata.Kind, name string) (context.Context, error) {
	objectID, err := applicationObjectID(catalog, objectKind, name)
	if err != nil {
		return nil, err
	}
	permissions, err := runtime.applicationPermissions(ctx, databaseID, userID, catalog)
	if err != nil {
		return nil, err
	}
	if err := permissions.RequireObject(objectID, metadata.PermissionRead); err != nil {
		return nil, err
	}
	ctx = metadata.WithSessionValues(metadata.WithPermissions(ctx, permissions), applicationSessionValues(userID))
	return metadata.WithSessionResolver(ctx, applicationSessionResolver(snapshot, pool, catalog, userID)), nil
}

// LoadApplicationForm resolves generated or custom managed-form metadata.
func (runtime *Runtime) LoadApplicationForm(ctx context.Context, token string, databaseID uuid.UUID, objectKind metadata.Kind, name string, formKind metadata.FormKind, preferences []string) (ApplicationForm, error) {
	session, err := runtime.ResumePortalDatabase(ctx, token, databaseID)
	if err != nil {
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
	if _, err := runtime.requireApplicationRead(ctx, databaseID, session.UserID, snapshot, pool, catalog, objectKind, name); err != nil {
		return ApplicationForm{}, err
	}
	language := ApplicationLanguage(catalog.Project, preferences)
	var descriptor metadata.FormDescriptor
	switch objectKind {
	case metadata.CatalogKind:
		descriptor, err = catalog.CatalogForm(name, formKind, language.Code)
	case metadata.DocumentKind:
		descriptor, err = catalog.DocumentForm(name, formKind, language.Code)
	default:
		err = fmt.Errorf("unsupported application object kind %q", objectKind)
	}
	if err != nil {
		return ApplicationForm{}, err
	}
	result := ApplicationForm{Descriptor: descriptor, Language: language}
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
	session, err := runtime.ResumePortalDatabase(ctx, token, databaseID)
	if err != nil {
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
	// The context carries the policy onward so row filtering can use it without
	// resolving the assignment a second time.
	ctx, err = runtime.requireApplicationRead(ctx, databaseID, session.UserID, snapshot, pool, catalog, objectKind, name)
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
	snapshot, found, err := publication.CurrentDatabaseState(ctx, pool)
	if err != nil || !found {
		pool.Close()
		if err != nil {
			return metadata.RuntimeSnapshot{}, nil, err
		}
		return metadata.RuntimeSnapshot{}, nil, fmt.Errorf("application database has no saved data yet")
	}
	if err := snapshot.Validate(); err != nil {
		pool.Close()
		return metadata.RuntimeSnapshot{}, nil, fmt.Errorf("validate saved database state: %w", err)
	}
	return snapshot, pool, nil
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

func resolvedApplicationTitle(title metadata.LocalizedText, fallback string, language metadata.TitleLanguage) string {
	if value := language.Resolve(title); value != "" {
		return value
	}
	return fallback
}

// ApplicationLanguage picks the language one reader sees an application in:
// their first preference the project actually has, otherwise the project's own
// default. The choice is made here, where the project's languages are known -
// a caller holding only an HTTP header cannot make it correctly.
//
// Preferences come from the request today; when accounts carry a language of
// their own (092) that value goes in front of them, and nothing else changes.
func ApplicationLanguage(manifest project.Project, preferences []string) metadata.TitleLanguage {
	for _, preference := range preferences {
		code := strings.ToLower(strings.TrimSpace(preference))
		if code == "" {
			continue
		}
		for _, language := range manifest.Languages {
			if strings.EqualFold(language.Code, code) {
				return metadata.ProjectLanguage(manifest, language.Code)
			}
		}
		base, _, _ := strings.Cut(code, "-")
		for _, language := range manifest.Languages {
			if configured, _, _ := strings.Cut(strings.ToLower(language.Code), "-"); configured == base {
				return metadata.ProjectLanguage(manifest, language.Code)
			}
		}
	}
	return metadata.ProjectLanguage(manifest, manifest.DefaultLanguage)
}

// allowedOperations reports what the caller may do with one object, in a stable
// order. It answers only about the object as a whole: a restriction on rows or
// on fields narrows what a granted operation reaches, never whether it exists.
func allowedOperations(permissions *metadata.Permissions, objectID uuid.UUID) []metadata.PermissionOperation {
	all := []metadata.PermissionOperation{
		metadata.PermissionRead, metadata.PermissionCreate, metadata.PermissionUpdate,
		metadata.PermissionDelete, metadata.PermissionPost, metadata.PermissionUndoPosting,
	}
	result := make([]metadata.PermissionOperation, 0, len(all))
	for _, operation := range all {
		if permissions.AllowsObject(objectID, operation) {
			result = append(result, operation)
		}
	}
	return result
}

// ApplicationPublication reports what the caller's database is currently saved
// as. ML App polls it while a session is open so that a configuration saved
// while someone is working shows up as an offer to refresh rather than as a
// form that silently no longer matches the data behind it.
func (runtime *Runtime) ApplicationPublication(ctx context.Context, token string, databaseID uuid.UUID) (publication.PublicationMarker, error) {
	if _, err := runtime.ResumePortalDatabase(ctx, token, databaseID); err != nil {
		return publication.PublicationMarker{}, err
	}
	pool, _, err := runtime.openApplicationPool(ctx, databaseID)
	if err != nil {
		return publication.PublicationMarker{}, err
	}
	defer pool.Close()
	marker, _, err := publication.CurrentPublicationMarker(ctx, pool)
	if err != nil {
		return publication.PublicationMarker{}, err
	}
	return marker, nil
}
