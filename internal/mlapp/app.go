// Package mlapp provides the browser component shell shared by application forms.
package mlapp

import (
	"crypto/sha256"
	"embed"
	"fmt"
	"net/http"
	"path"
	"strings"

	"github.com/k33alexey/MetaLab/internal/systemdb"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

const BootstrapFormat = 1

//go:embed ui/*
var assets embed.FS

type Bootstrap struct {
	Format     int              `json:"format"`
	Database   Database         `json:"database"`
	User       User             `json:"user"`
	Locale     string           `json:"locale"`
	Navigation []NavigationItem `json:"navigation"`
	Form       Form             `json:"form"`
}

type Database struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

type User struct {
	ID    uuid.UUID `json:"id"`
	Login string    `json:"login"`
}

type NavigationItem struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Current bool   `json:"current,omitempty"`
}

type Form struct {
	ID       string    `json:"id"`
	Title    string    `json:"title"`
	Commands []Command `json:"commands"`
	Items    []Element `json:"items"`
}

type Command struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Kind     string `json:"kind,omitempty"`
	Disabled bool   `json:"disabled,omitempty"`
}

type Element struct {
	ID          string    `json:"id"`
	Kind        string    `json:"kind"`
	Title       string    `json:"title,omitempty"`
	Value       string    `json:"value,omitempty"`
	InputType   string    `json:"inputType,omitempty"`
	ReadOnly    bool      `json:"readOnly,omitempty"`
	Disabled    bool      `json:"disabled,omitempty"`
	Orientation string    `json:"orientation,omitempty"`
	Command     string    `json:"command,omitempty"`
	Children    []Element `json:"children,omitempty"`
}

// NewBootstrap creates the minimal authenticated application workspace.
func NewBootstrap(databaseID uuid.UUID, session systemdb.DatabaseSession) Bootstrap {
	return Bootstrap{
		Format:     BootstrapFormat,
		Database:   Database{ID: databaseID, Name: session.DatabaseName},
		User:       User{ID: session.UserID, Login: session.Login},
		Locale:     "ru",
		Navigation: []NavigationItem{{ID: "home", Title: "Главное", Current: true}},
		Form: Form{
			ID: "home", Title: "Начальная страница",
			Commands: []Command{{ID: "refresh", Title: "Обновить", Kind: "secondary"}},
			Items: []Element{{
				ID: "welcome", Kind: "group", Title: "ML App", Orientation: "vertical",
				Children: []Element{{ID: "database", Kind: "label", Title: "База", Value: session.DatabaseName}},
			}},
		},
	}
}

// ServePage returns the CSP-compatible application document.
func ServePage(response http.ResponseWriter) {
	serveAsset(response, nil, "index.html", "text/html; charset=utf-8", "no-store")
}

// ServeAsset returns one embedded, path-bounded ML App asset.
func ServeAsset(response http.ResponseWriter, request *http.Request, name string) {
	name = path.Base(name)
	switch name {
	case "app.css":
		serveAsset(response, request, name, "text/css; charset=utf-8", "no-cache")
	case "app.js":
		serveAsset(response, request, name, "text/javascript; charset=utf-8", "no-cache")
	default:
		http.Error(response, "Not found", http.StatusNotFound)
	}
}

func serveAsset(response http.ResponseWriter, request *http.Request, name, contentType, cacheControl string) {
	content, err := assets.ReadFile("ui/" + name)
	if err != nil {
		http.Error(response, "ML App asset unavailable", http.StatusInternalServerError)
		return
	}
	digest := sha256.Sum256(content)
	etag := fmt.Sprintf(`"ml-app-%x"`, digest[:12])
	response.Header().Set("ETag", etag)
	response.Header().Set("Cache-Control", cacheControl)
	if request != nil && request.Header.Get("If-None-Match") == etag {
		response.WriteHeader(http.StatusNotModified)
		return
	}
	response.Header().Set("Content-Type", contentType)
	_, _ = response.Write(content)
}

// Validate checks the bounded component contract before it reaches a browser.
func (bootstrap Bootstrap) Validate() error {
	if bootstrap.Format != BootstrapFormat || bootstrap.Database.ID.IsZero() || bootstrap.User.ID.IsZero() {
		return fmt.Errorf("invalid ML App bootstrap identity")
	}
	if strings.TrimSpace(bootstrap.Database.Name) == "" || strings.TrimSpace(bootstrap.User.Login) == "" {
		return fmt.Errorf("invalid ML App bootstrap labels")
	}
	if len(bootstrap.Navigation) > 1_000 {
		return fmt.Errorf("ML App navigation exceeds 1000 items")
	}
	if err := validateForm(bootstrap.Form); err != nil {
		return err
	}
	return nil
}

func validateForm(form Form) error {
	if form.ID == "" || form.Title == "" || len(form.Commands) > 1_024 {
		return fmt.Errorf("invalid ML App form")
	}
	commands := make(map[string]bool, len(form.Commands))
	for _, command := range form.Commands {
		if command.ID == "" || command.Title == "" || commands[command.ID] {
			return fmt.Errorf("invalid ML App command")
		}
		commands[command.ID] = true
	}
	stack, count := append([]Element(nil), form.Items...), 0
	for len(stack) != 0 {
		item := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		count++
		if count > 10_000 || item.ID == "" {
			return fmt.Errorf("invalid ML App form element")
		}
		switch item.Kind {
		case "group", "field", "label", "table", "button":
		default:
			return fmt.Errorf("unsupported ML App component %q", item.Kind)
		}
		if item.Kind == "button" && !commands[item.Command] {
			return fmt.Errorf("ML App button references an unknown command")
		}
		stack = append(stack, item.Children...)
	}
	return nil
}
