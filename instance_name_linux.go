//go:build linux

package main

import "buildflow/internal/appdata"

func instanceName(store *appdata.Store) string { return store.IdentityKey() }
