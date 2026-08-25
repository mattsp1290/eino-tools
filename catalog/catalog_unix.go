//go:build unix

package catalog

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"

	"github.com/mattsp1290/eino-tools/applypatch"
	"github.com/mattsp1290/eino-tools/fileops"
	"github.com/mattsp1290/eino-tools/glob"
	"github.com/mattsp1290/eino-tools/search"
	"github.com/mattsp1290/eino-tools/shell"
	"github.com/mattsp1290/eino-tools/trackerwrite"
	"github.com/mattsp1290/eino-tools/urlfetch"
	"github.com/mattsp1290/eino-tools/userinteract"
)

type definitionSpec struct {
	id           string
	revision     int
	name         string
	binding      BindingKind
	retrySafe    bool
	concurrent   bool
	dependencies []executorDependency
	environment  string
	info         func() (*schema.ToolInfo, error)
	newTool      func(context.Context, Instance) (tool.InvokableTool, error)
}

// Standard returns the complete validated standard catalog in stable order.
func Standard(options Options) ([]Definition, error) {
	searchOptions, searchExecutable, err := captureSearchOptions(options.SearchOptions)
	if err != nil {
		return nil, err
	}
	shellOptions, shellExecutable, err := captureShellOptions(options.ShellOptions)
	if err != nil {
		return nil, err
	}

	userSurface := options.UserSurface
	if userSurface == "" {
		userSurface = userinteract.SurfaceMCP
	}
	if userSurface != userinteract.SurfaceCLI && userSurface != userinteract.SurfaceMCP {
		return nil, fmt.Errorf("catalog: invalid user interaction surface %q", userSurface)
	}
	userOptions := options.UserOptions
	if userOptions.Stdin == nil {
		userOptions.Stdin = os.Stdin
	}
	if userOptions.Stderr == nil {
		userOptions.Stderr = os.Stderr
	}
	if userSurface == userinteract.SurfaceCLI {
		if isNilLike(userOptions.Stdin) {
			return nil, errors.New("catalog: CLI stdin has a nil dynamic value")
		}
		if isNilLike(userOptions.Stderr) {
			return nil, errors.New("catalog: CLI stderr has a nil dynamic value")
		}
	}

	trackerWriter := options.TrackerWriter
	if trackerWriter != nil && isNilLike(trackerWriter) {
		return nil, errors.New("catalog: tracker writer has a nil dynamic value")
	}

	var urlOptions *urlfetch.Options
	if options.URLFetchOptions != nil {
		copy := *options.URLFetchOptions
		urlOptions = &copy
	}

	workspace := func(construct func(string) (tool.InvokableTool, error)) func(context.Context, Instance) (tool.InvokableTool, error) {
		return func(ctx context.Context, instance Instance) (tool.InvokableTool, error) {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			root, err := validateWorkspaceRoot(instance.WorkspaceRoot)
			if err != nil {
				return nil, err
			}
			return construct(root)
		}
	}

	searchFactory := workspace(func(root string) (tool.InvokableTool, error) {
		if err := verifyExecutable(searchExecutable); err != nil {
			return nil, err
		}
		return search.New(root, searchOptions)
	})

	shellFactory := workspace(func(root string) (tool.InvokableTool, error) {
		if err := verifyExecutable(shellExecutable); err != nil {
			return nil, err
		}
		return shell.New(root, shellOptions)
	})

	static := func(construct func() (tool.InvokableTool, error)) func(context.Context, Instance) (tool.InvokableTool, error) {
		return func(ctx context.Context, _ Instance) (tool.InvokableTool, error) {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			return construct()
		}
	}
	urlFactory := static(func() (tool.InvokableTool, error) {
		if urlOptions == nil {
			return urlfetch.New()
		}
		return urlfetch.New(*urlOptions)
	})
	userFactory := static(func() (tool.InvokableTool, error) { return userinteract.New(userSurface, userOptions) })

	specs := []definitionSpec{
		{id: IDFileRead, revision: 1, name: fileops.NameRead, binding: BindingWorkspace, retrySafe: true, info: fileops.ReadToolInfo,
			newTool: workspace(func(root string) (tool.InvokableTool, error) { return fileops.NewReadTool(root) })},
		{id: IDFileWrite, revision: 1, name: fileops.NameWrite, binding: BindingWorkspace, info: fileops.WriteToolInfo,
			newTool: workspace(func(root string) (tool.InvokableTool, error) { return fileops.NewWriteTool(root) })},
		{id: IDFileEdit, revision: 1, name: fileops.NameEdit, binding: BindingWorkspace, info: fileops.EditToolInfo,
			newTool: workspace(func(root string) (tool.InvokableTool, error) { return fileops.NewEditTool(root) })},
		{id: IDFileList, revision: 1, name: fileops.NameList, binding: BindingWorkspace, retrySafe: true, info: fileops.ListToolInfo,
			newTool: workspace(func(root string) (tool.InvokableTool, error) { return fileops.NewListTool(root) })},
		{id: IDGlob, revision: 1, name: glob.Name, binding: BindingWorkspace, retrySafe: true, info: glob.ToolInfo,
			newTool: workspace(func(root string) (tool.InvokableTool, error) { return glob.New(root) })},
		{id: IDSearch, revision: 1, name: search.Name, binding: BindingWorkspace, retrySafe: true,
			dependencies: searchExecutable.dependencies(), environment: searchExecutable.environment, info: search.ToolInfo, newTool: searchFactory},
		{id: IDApplyPatch, revision: 1, name: applypatch.Name, binding: BindingWorkspace, info: applypatch.ToolInfo,
			newTool: workspace(func(root string) (tool.InvokableTool, error) { return applypatch.New(root) })},
		{id: IDShell, revision: 1, name: shell.Name, binding: BindingWorkspace,
			dependencies: shellExecutable.dependencies(), environment: shellExecutable.environment, info: shell.ToolInfo, newTool: shellFactory},
		{id: IDURLFetch, revision: 1, name: urlfetch.Name, binding: BindingStatic, retrySafe: true, concurrent: true,
			info: urlfetch.ToolInfo, newTool: urlFactory},
		{id: IDUserInteract, revision: 1, name: userinteract.Name, binding: BindingStatic,
			info: userinteract.ToolInfo, newTool: userFactory},
	}
	if trackerWriter != nil {
		trackerFactory := static(func() (tool.InvokableTool, error) { return trackerwrite.New(trackerWriter) })
		specs = append(specs, definitionSpec{
			id: IDTrackerWrite, revision: 1, name: trackerwrite.Name, binding: BindingStatic,
			info: trackerwrite.ToolInfo, newTool: trackerFactory,
		})
	}

	definitions := make([]Definition, 0, len(specs))
	for _, spec := range specs {
		definition, err := makeDefinition(spec)
		if err != nil {
			return nil, err
		}
		definitions = append(definitions, definition)
	}

	if err := validateDefinitions(definitions); err != nil {
		return nil, err
	}
	return definitions, nil
}

func makeDefinition(spec definitionSpec) (Definition, error) {
	if spec.revision <= 0 {
		return Definition{}, fmt.Errorf("catalog: executor revision for %s must be positive", spec.id)
	}
	if spec.info == nil {
		return Definition{}, fmt.Errorf("catalog: metadata accessor for %s is nil", spec.id)
	}
	if spec.newTool == nil {
		return Definition{}, fmt.Errorf("catalog: factory for %s is nil", spec.id)
	}
	metadata, err := spec.info()
	if err != nil {
		return Definition{}, fmt.Errorf("catalog: metadata for %s: %w", spec.id, err)
	}
	schemaIdentity, err := schemaHash(metadata)
	if err != nil {
		return Definition{}, fmt.Errorf("catalog: schema identity for %s: %w", spec.id, err)
	}
	executorIdentity, err := executorHash(spec.id, spec.revision, spec.dependencies, spec.environment)
	if err != nil {
		return Definition{}, fmt.Errorf("catalog: executor identity for %s: %w", spec.id, err)
	}
	return Definition{
		ID: spec.id, Name: spec.name, Binding: spec.binding, RetrySafe: spec.retrySafe, Concurrent: spec.concurrent,
		SchemaHash: schemaIdentity, ExecutorHash: executorIdentity, Info: spec.info, New: spec.newTool,
	}, nil
}

func validateDefinitions(definitions []Definition) error {
	ids := make(map[string]struct{}, len(definitions))
	names := make(map[string]struct{}, len(definitions))
	for i, definition := range definitions {
		if definition.Binding != BindingStatic && definition.Binding != BindingWorkspace {
			return fmt.Errorf("catalog: definition %d has unknown binding %q", i, definition.Binding)
		}
		if definition.ID == "" || definition.Name == "" {
			return fmt.Errorf("catalog: definition %d has empty ID or name", i)
		}
		if _, exists := ids[definition.ID]; exists {
			return fmt.Errorf("catalog: duplicate ID %q", definition.ID)
		}
		ids[definition.ID] = struct{}{}
		if _, exists := names[definition.Name]; exists {
			return fmt.Errorf("catalog: duplicate name %q", definition.Name)
		}
		names[definition.Name] = struct{}{}
		if !validHash(definition.SchemaHash) || !validHash(definition.ExecutorHash) {
			return fmt.Errorf("catalog: definition %q has invalid identity hash", definition.ID)
		}
		if definition.Info == nil || definition.New == nil {
			return fmt.Errorf("catalog: definition %q has nil metadata accessor or factory", definition.ID)
		}
		metadata, err := definition.Info()
		if err != nil {
			return fmt.Errorf("catalog: metadata for %q: %w", definition.ID, err)
		}
		if metadata == nil || metadata.Name != definition.Name || metadata.ParamsOneOf == nil {
			return fmt.Errorf("catalog: metadata mismatch for %q", definition.ID)
		}
		actualSchemaHash, err := schemaHash(metadata)
		if err != nil {
			return fmt.Errorf("catalog: validate schema for %q: %w", definition.ID, err)
		}
		if actualSchemaHash != definition.SchemaHash {
			return fmt.Errorf("catalog: schema identity mismatch for %q", definition.ID)
		}
	}
	return nil
}

func validHash(value string) bool {
	if len(value) != sha256.Size*2 || value != strings.ToLower(value) {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func validateWorkspaceRoot(root string) (string, error) {
	if root == "" {
		return "", errors.New("catalog: workspace root is required")
	}
	if !filepath.IsAbs(root) {
		return "", fmt.Errorf("catalog: workspace root must be absolute, got %q", root)
	}
	clean := filepath.Clean(root)
	if clean != root {
		return "", fmt.Errorf("catalog: workspace root is not canonical: %q", root)
	}
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", fmt.Errorf("catalog: resolve workspace root %q: %w", root, err)
	}
	if resolved != root {
		return "", fmt.Errorf("catalog: workspace root is not canonical: %q", root)
	}
	info, err := os.Stat(root)
	if err != nil {
		return "", fmt.Errorf("catalog: stat workspace root %q: %w", root, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("catalog: workspace root is not a directory: %q", root)
	}
	return root, nil
}

func isNilLike(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}
