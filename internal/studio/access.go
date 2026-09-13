package studio

import (
	"context"
	"crypto/subtle"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/k33alexey/MetaLab/internal/auth"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// NewAuthorizedHandler gives one desktop WebView access to an authenticated Studio lease.
// The returned single-use launch path is replaced by an HttpOnly cookie, never by a JS token.
func NewAuthorizedHandler(workspace *Workspace, host string, authorize func(context.Context) error) (http.Handler, string, error) {
	return protectStudioHandler(NewHandler(workspace), host, authorize)
}

func protectStudioHandler(next http.Handler, host string, authorize func(context.Context) error) (http.Handler, string, error) {
	if host == "" || authorize == nil {
		return nil, "", fmt.Errorf("Studio host and authorization are required")
	}
	launch, launchDigest, err := auth.NewSessionToken()
	if err != nil {
		return nil, "", err
	}
	session, sessionDigest, err := auth.NewSessionToken()
	if err != nil {
		return nil, "", err
	}
	cookieName := "ml_studio_" + uuid.MustNew().String()
	const prefix = "/__ml_open/"
	expires := time.Now().Add(time.Minute)
	var mu sync.Mutex
	used := false
	guard := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if r.Host != host {
			http.Error(w, "Invalid Studio host", 403)
			return
		}
		bootstrap := r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, prefix)
		if bootstrap {
			digest := auth.SessionTokenDigest(strings.TrimPrefix(r.URL.Path, prefix))
			mu.Lock()
			valid := !used && time.Now().Before(expires) && subtle.ConstantTimeCompare(digest[:], launchDigest[:]) == 1
			if valid {
				used = true
			}
			mu.Unlock()
			if !valid {
				http.Error(w, "Open Studio from ML Manager", 401)
				return
			}
		} else {
			cookie, err := r.Cookie(cookieName)
			if err != nil {
				http.Error(w, "Open Studio from ML Manager", 401)
				return
			}
			digest := auth.SessionTokenDigest(cookie.Value)
			if subtle.ConstantTimeCompare(digest[:], sessionDigest[:]) != 1 {
				http.Error(w, "Open Studio from ML Manager", 401)
				return
			}
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		err := authorize(ctx)
		cancel()
		if err != nil {
			http.Error(w, "Studio session is no longer authorized", 403)
			return
		}
		if bootstrap {
			http.SetCookie(w, &http.Cookie{Name: cookieName, Value: session, Path: "/", HttpOnly: true, Secure: r.TLS != nil, SameSite: http.SameSiteStrictMode})
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	})
	return http.NewCrossOriginProtection().Handler(guard), prefix + launch, nil
}
