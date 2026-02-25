// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package sourcemap

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/dotandev/hintents/internal/logger"
)

// Resolver coordinates fetching verified source code from a registry,
// with optional local caching. It is the primary entry point for
// downstream consumers that need contract source code.
type Resolver struct {
	registry *RegistryClient
	cache    *SourceCache
}

// ResolverOption is a functional option for configuring the Resolver.
type ResolverOption func(*Resolver)

// WithCache enables caching with the specified directory.
func WithCache(cacheDir string) ResolverOption {
	return func(r *Resolver) {
		cache, err := NewSourceCache(filepath.Join(cacheDir, "sourcemap"))
		if err != nil {
			logger.Logger.Warn("Failed to create source cache, caching disabled", "error", err)
			return
		}
		r.cache = cache
	}
}

// WithRegistryClient sets a custom registry client.
func WithRegistryClient(rc *RegistryClient) ResolverOption {
	return func(r *Resolver) {
		r.registry = rc
	}
}

// NewResolver creates a Resolver with the given options.
func NewResolver(opts ...ResolverOption) *Resolver {
	r := &Resolver{
		registry: NewRegistryClient(),
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// Resolve attempts to find verified source code for the given contract ID.
// It checks the local cache first, then queries the registry.
//
// Returns nil with no error if no verified source is available.
func (r *Resolver) Resolve(ctx context.Context, contractID string) (*SourceCode, error) {
	if err := validateContractID(contractID); err != nil {
		return nil, fmt.Errorf("invalid contract ID: %w", err)
	}

	// Check cache first
	if r.cache != nil {
		if cached := r.cache.Get(contractID); cached != nil {
			logger.Logger.Info("Source resolved from cache", "contract_id", contractID)
			return cached, nil
		}
	}

	// Fetch from registry
	source, err := r.registry.FetchVerifiedSource(ctx, contractID)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve source for %s: %w", contractID, err)
	}

	if source == nil {
		logger.Logger.Debug("No verified source available", "contract_id", contractID)
		return nil, nil
	}

	// Cache the result
	if r.cache != nil {
		if err := r.cache.Put(source); err != nil {
			logger.Logger.Warn("Failed to cache source", "contract_id", contractID, "error", err)
		}
	}

	logger.Logger.Info("Source resolved from registry",
		"contract_id", contractID,
		"repository", source.Repository,
		"file_count", len(source.Files),
	)

	return source, nil
}

// InvalidateCache removes a specific contract from the cache.
func (r *Resolver) InvalidateCache(contractID string) error {
	if r.cache == nil {
		return nil
	}
	return r.cache.Invalidate(contractID)
}

// ClearCache removes all cached source entries.
func (r *Resolver) ClearCache() error {
	if r.cache == nil {
		return nil
	}
	return r.cache.Clear()
}
