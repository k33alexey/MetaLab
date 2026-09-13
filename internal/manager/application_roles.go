package manager

import (
	"context"
	"errors"
	"net/http"

	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/platform"
	"github.com/k33alexey/MetaLab/internal/systemdb"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

type applicationRoleBackend interface {
	GetManagerApplicationRoles(context.Context, uuid.UUID, uuid.UUID) (platform.ManagerApplicationRoles, error)
	SetManagerApplicationRoles(context.Context, uuid.UUID, uuid.UUID, platform.ApplicationRoleUpdate) (systemdb.ApplicationRoleAssignment, error)
}

// Routes are wrapped by secureManager: cookie authentication, current database
// administration rights and cross-origin protection apply to reads and writes.
func registerApplicationRoleRoutes(routes *http.ServeMux, backend platformSetup) {
	roles, available := backend.(applicationRoleBackend)
	serve := func(w http.ResponseWriter, r *http.Request) {
		if !available {
			http.Error(w, "Application role administration unavailable", http.StatusServiceUnavailable)
			return
		}
		databaseID, ok := parsePathUUID(w, r, "database")
		if !ok {
			return
		}
		userID, err := uuid.Parse(r.PathValue("userID"))
		if err != nil {
			http.Error(w, "Invalid user identifier", http.StatusBadRequest)
			return
		}
		if r.Method == http.MethodGet {
			view, err := roles.GetManagerApplicationRoles(r.Context(), databaseID, userID)
			if err != nil {
				applicationRoleError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, view)
			return
		}
		var input platform.ApplicationRoleUpdate
		if !decodeJSONLimit(w, r, &input, 64<<10) {
			return
		}
		assignment, err := roles.SetManagerApplicationRoles(r.Context(), databaseID, userID, input)
		if err != nil {
			applicationRoleError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, assignment)
	}
	routes.HandleFunc("GET /api/databases/{id}/application-roles/{userID}", serve)
	routes.HandleFunc("PUT /api/databases/{id}/application-roles/{userID}", serve)
}

func applicationRoleError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, systemdb.ErrApplicationRolesChanged):
		http.Error(w, "Role assignment changed; reload before saving", http.StatusConflict)
	case errors.Is(err, systemdb.ErrInvalidApplicationRoles), errors.Is(err, metadata.ErrInvalidRoleSelection):
		http.Error(w, "Invalid role selection; use roles from the active publication", http.StatusBadRequest)
	default:
		managerAccessError(w, err)
	}
}
