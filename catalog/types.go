package catalog

import (
	"context"
	"errors"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"

	"github.com/mattsp1290/eino-tools/search"
	"github.com/mattsp1290/eino-tools/shell"
	"github.com/mattsp1290/eino-tools/tracker"
	"github.com/mattsp1290/eino-tools/urlfetch"
	"github.com/mattsp1290/eino-tools/userinteract"
)

// BindingKind describes the host input required to materialize a leaf tool.
// It is independent of host mount scope, permissions, and component lifetime.
type BindingKind string

const (
	BindingStatic    BindingKind = "static"
	BindingWorkspace BindingKind = "workspace"
)

const (
	IDFileRead     = "standard.file-read"
	IDFileWrite    = "standard.file-write"
	IDFileEdit     = "standard.file-edit"
	IDFileList     = "standard.file-list"
	IDGlob         = "standard.glob"
	IDSearch       = "standard.search"
	IDApplyPatch   = "standard.apply-patch"
	IDShell        = "standard.shell"
	IDURLFetch     = "standard.url-fetch"
	IDUserInteract = "standard.user-interact"
	IDTrackerWrite = "standard.tracker-write"
)

// ErrUnsupportedPlatform reports that the standard executable catalog is not
// available on the current platform.
var ErrUnsupportedPlatform = errors.New("catalog: standard tools are unsupported on this platform")

// Instance supplies host-admitted materialization inputs.
type Instance struct {
	WorkspaceRoot string
}

// Definition is a deterministic description and fresh factory for one leaf
// tool. Hosts attach their own policy and lifecycle identity when translating
// definitions into runtime registrations.
type Definition struct {
	ID           string
	Name         string
	Binding      BindingKind
	RetrySafe    bool
	Concurrent   bool
	SchemaHash   string
	ExecutorHash string
	Info         func() (*schema.ToolInfo, error)
	New          func(context.Context, Instance) (tool.InvokableTool, error)
}

// Options captures host-owned dependencies when Standard is called.
type Options struct {
	SearchOptions   *search.Options
	ShellOptions    *shell.Options
	URLFetchOptions *urlfetch.Options
	UserSurface     userinteract.Surface
	UserOptions     userinteract.Options
	TrackerWriter   tracker.CloseWriter
}
