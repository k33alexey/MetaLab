//go:build !darwin && !linux

package systemservice

import kservice "github.com/kardianos/service"

func configureServiceIdentity(*kservice.Config) {}

func prepareInstall(string, string) error { return nil }
