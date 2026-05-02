package provider_test

import (
	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/provider"
	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/provider/cloudrunsandbox"
	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/provider/dockergce"
	"github.com/pollenjp/cc-tunnel/apps/cc-tunnel/internal/provider/local"
)

// Compile-time interface satisfaction checks.
var _ provider.ExecutionProvider = (*local.Provider)(nil)
var _ provider.ExecutionProvider = (*local.LocalDockerProvider)(nil)
var _ provider.ExecutionProvider = (*cloudrunsandbox.MockProvider)(nil)
var _ provider.ExecutionProvider = (*dockergce.MockProvider)(nil)
