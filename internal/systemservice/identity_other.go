//go:build !darwin && !linux

package systemservice

import kservice "github.com/kardianos/service"

func configureServiceIdentity(*kservice.Config) {}
