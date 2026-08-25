//go:build unix

package catalog

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
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

const (
	revisionFileRead     = 1
	revisionFileWrite    = 1
	revisionFileEdit     = 1
	revisionFileList     = 1
	revisionGlob         = 1
	revisionSearch       = 1
	revisionApplyPatch   = 1
	revisionShell        = 1
	revisionURLFetch     = 1
	revisionUserInteract = 1
	revisionTrackerWrite = 1
)

type capturedExecutable struct {
	kind          string
	path          string
	contentSHA256 string
	environment   string
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

	definitions := make([]Definition, 0, 11)
	appendDefinition := func(def Definition, err error) error {
		if err != nil {
			return err
		}
		definitions = append(definitions, def)
		return nil
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

	if err := appendDefinition(makeDefinition(IDFileRead, fileops.NameRead, BindingWorkspace, true, false, nil, "", fileops.ReadToolInfo,
		workspace(func(root string) (tool.InvokableTool, error) { return fileops.NewReadTool(root) }))); err != nil {
		return nil, err
	}
	if err := appendDefinition(makeDefinition(IDFileWrite, fileops.NameWrite, BindingWorkspace, false, false, nil, "", fileops.WriteToolInfo,
		workspace(func(root string) (tool.InvokableTool, error) { return fileops.NewWriteTool(root) }))); err != nil {
		return nil, err
	}
	if err := appendDefinition(makeDefinition(IDFileEdit, fileops.NameEdit, BindingWorkspace, false, false, nil, "", fileops.EditToolInfo,
		workspace(func(root string) (tool.InvokableTool, error) { return fileops.NewEditTool(root) }))); err != nil {
		return nil, err
	}
	if err := appendDefinition(makeDefinition(IDFileList, fileops.NameList, BindingWorkspace, true, false, nil, "", fileops.ListToolInfo,
		workspace(func(root string) (tool.InvokableTool, error) { return fileops.NewListTool(root) }))); err != nil {
		return nil, err
	}
	if err := appendDefinition(makeDefinition(IDGlob, glob.Name, BindingWorkspace, true, false, nil, "", glob.ToolInfo,
		workspace(func(root string) (tool.InvokableTool, error) { return glob.New(root) }))); err != nil {
		return nil, err
	}

	searchFactory := workspace(func(root string) (tool.InvokableTool, error) {
		if err := verifyExecutable(searchExecutable); err != nil {
			return nil, err
		}
		return search.New(root, searchOptions)
	})
	if err := appendDefinition(makeDefinition(IDSearch, search.Name, BindingWorkspace, true, false,
		[]executorDependency{searchExecutable.dependency()}, searchExecutable.environment, search.ToolInfo, searchFactory)); err != nil {
		return nil, err
	}

	if err := appendDefinition(makeDefinition(IDApplyPatch, applypatch.Name, BindingWorkspace, false, false, nil, "", applypatch.ToolInfo,
		workspace(func(root string) (tool.InvokableTool, error) { return applypatch.New(root) }))); err != nil {
		return nil, err
	}

	shellFactory := workspace(func(root string) (tool.InvokableTool, error) {
		if err := verifyExecutable(shellExecutable); err != nil {
			return nil, err
		}
		return shell.New(root, shellOptions)
	})
	if err := appendDefinition(makeDefinition(IDShell, shell.Name, BindingWorkspace, false, false,
		[]executorDependency{shellExecutable.dependency()}, shellExecutable.environment, shell.ToolInfo, shellFactory)); err != nil {
		return nil, err
	}

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
	if err := appendDefinition(makeDefinition(IDURLFetch, urlfetch.Name, BindingStatic, true, true, nil, "", urlfetch.ToolInfo, urlFactory)); err != nil {
		return nil, err
	}
	userFactory := static(func() (tool.InvokableTool, error) { return userinteract.New(userSurface, userOptions) })
	if err := appendDefinition(makeDefinition(IDUserInteract, userinteract.Name, BindingStatic, false, false, nil, "", userinteract.ToolInfo, userFactory)); err != nil {
		return nil, err
	}
	if trackerWriter != nil {
		trackerFactory := static(func() (tool.InvokableTool, error) { return trackerwrite.New(trackerWriter) })
		if err := appendDefinition(makeDefinition(IDTrackerWrite, trackerwrite.Name, BindingStatic, false, false, nil, "", trackerwrite.ToolInfo, trackerFactory)); err != nil {
			return nil, err
		}
	}

	if err := validateDefinitions(definitions); err != nil {
		return nil, err
	}
	return definitions, nil
}

func makeDefinition(id, name string, binding BindingKind, retrySafe, concurrent bool,
	dependencies []executorDependency, environment string, info func() (*schema.ToolInfo, error),
	newTool func(context.Context, Instance) (tool.InvokableTool, error),
) (Definition, error) {
	metadata, err := info()
	if err != nil {
		return Definition{}, fmt.Errorf("catalog: metadata for %s: %w", id, err)
	}
	schemaIdentity, err := schemaHash(metadata)
	if err != nil {
		return Definition{}, fmt.Errorf("catalog: schema identity for %s: %w", id, err)
	}
	revision, err := executorRevision(id)
	if err != nil {
		return Definition{}, err
	}
	executorIdentity, err := executorHash(id, revision, dependencies, environment)
	if err != nil {
		return Definition{}, fmt.Errorf("catalog: executor identity for %s: %w", id, err)
	}
	return Definition{
		ID: id, Name: name, Binding: binding, RetrySafe: retrySafe, Concurrent: concurrent,
		SchemaHash: schemaIdentity, ExecutorHash: executorIdentity, Info: info, New: newTool,
	}, nil
}

func executorRevision(id string) (int, error) {
	switch id {
	case IDFileRead:
		return revisionFileRead, nil
	case IDFileWrite:
		return revisionFileWrite, nil
	case IDFileEdit:
		return revisionFileEdit, nil
	case IDFileList:
		return revisionFileList, nil
	case IDGlob:
		return revisionGlob, nil
	case IDSearch:
		return revisionSearch, nil
	case IDApplyPatch:
		return revisionApplyPatch, nil
	case IDShell:
		return revisionShell, nil
	case IDURLFetch:
		return revisionURLFetch, nil
	case IDUserInteract:
		return revisionUserInteract, nil
	case IDTrackerWrite:
		return revisionTrackerWrite, nil
	default:
		return 0, fmt.Errorf("catalog: no executor revision for %q", id)
	}
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

func captureSearchOptions(input *search.Options) (search.Options, capturedExecutable, error) {
	options := search.Options{}
	if input != nil {
		options = *input
		options.Env = append([]string(nil), input.Env...)
	}
	if options.RGBinary == "" {
		options.RGBinary = "rg"
	}
	if options.Env == nil {
		options.Env = os.Environ()
	}
	env, err := normalizeEnvironment(options.Env)
	if err != nil {
		return search.Options{}, capturedExecutable{}, fmt.Errorf("catalog: search environment: %w", err)
	}
	path, digest, err := resolveExecutable(options.RGBinary, env)
	if err != nil {
		return search.Options{}, capturedExecutable{}, fmt.Errorf("catalog: search executable: %w", err)
	}
	envDigest, err := hashJSON(env)
	if err != nil {
		return search.Options{}, capturedExecutable{}, err
	}
	options.RGBinary = path
	options.Env = env
	options.DisableConfig = true
	return options, capturedExecutable{kind: "ripgrep", path: path, contentSHA256: digest, environment: envDigest}, nil
}

func captureShellOptions(input *shell.Options) (shell.Options, capturedExecutable, error) {
	options := shell.Options{}
	if input != nil {
		options = *input
		options.Env = append([]string(nil), input.Env...)
	}
	if options.OutputCapBytes < 0 {
		return shell.Options{}, capturedExecutable{}, fmt.Errorf("catalog: shell output cap bytes must be non-negative, got %d", options.OutputCapBytes)
	}
	if options.OutputCapBytes == 0 {
		options.OutputCapBytes = shell.DefaultOutputCapBytes
	}
	if options.ShellBinary == "" {
		options.ShellBinary = shell.DefaultShellBinary
	}
	if options.Env == nil {
		options.Env = os.Environ()
	}
	env, err := normalizeEnvironment(options.Env)
	if err != nil {
		return shell.Options{}, capturedExecutable{}, fmt.Errorf("catalog: shell environment: %w", err)
	}
	path, digest, err := resolveExecutable(options.ShellBinary, env)
	if err != nil {
		return shell.Options{}, capturedExecutable{}, fmt.Errorf("catalog: shell executable: %w", err)
	}
	envDigest, err := hashJSON(env)
	if err != nil {
		return shell.Options{}, capturedExecutable{}, err
	}
	options.ShellBinary = path
	options.Env = env
	return options, capturedExecutable{kind: "shell", path: path, contentSHA256: digest, environment: envDigest}, nil
}

func normalizeEnvironment(entries []string) ([]string, error) {
	values := make(map[string]string, len(entries))
	for _, entry := range entries {
		if strings.ContainsRune(entry, 0) {
			return nil, errors.New("environment entry contains NUL byte")
		}
		key, value, ok := strings.Cut(entry, "=")
		if !ok || key == "" || strings.Contains(key, "=") {
			return nil, errors.New("environment entry must have a non-empty key and '=' separator")
		}
		values[key] = value
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	normalized := make([]string, 0, len(keys))
	for _, key := range keys {
		normalized = append(normalized, key+"="+values[key])
	}
	return normalized, nil
}

func resolveExecutable(name string, environment []string) (string, string, error) {
	if strings.ContainsRune(name, 0) || name == "" {
		return "", "", errors.New("executable name is empty or contains NUL")
	}
	var candidates []string
	if filepath.IsAbs(name) {
		candidates = []string{name}
	} else {
		if strings.ContainsRune(name, filepath.Separator) {
			return "", "", fmt.Errorf("relative executable path %q contains a separator", name)
		}
		pathValue := environmentValue(environment, "PATH")
		for _, directory := range filepath.SplitList(pathValue) {
			if directory == "" || !filepath.IsAbs(directory) {
				continue
			}
			candidates = append(candidates, filepath.Join(directory, name))
		}
	}
	for _, candidate := range candidates {
		resolved, err := filepath.EvalSymlinks(candidate)
		if err != nil {
			continue
		}
		info, err := os.Stat(resolved)
		if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
			continue
		}
		digest, err := fileDigest(resolved)
		if err != nil {
			continue
		}
		return resolved, digest, nil
	}
	return "", "", fmt.Errorf("executable %q was not found as a readable regular executable", name)
}

func environmentValue(environment []string, key string) string {
	prefix := key + "="
	for _, entry := range environment {
		if strings.HasPrefix(entry, prefix) {
			return strings.TrimPrefix(entry, prefix)
		}
	}
	return ""
}

func fileDigest(path string) (string, error) {
	file, err := os.Open(path) //nolint:gosec // path is host configuration, not model input
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func (executable capturedExecutable) dependency() executorDependency {
	return executorDependency{Kind: executable.kind, Path: executable.path, ContentSHA256: executable.contentSHA256}
}

func verifyExecutable(executable capturedExecutable) error {
	info, err := os.Stat(executable.path)
	if err != nil {
		return fmt.Errorf("catalog: verify %s executable: %w", executable.kind, err)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
		return fmt.Errorf("catalog: %s executable is no longer a regular executable at %q", executable.kind, executable.path)
	}
	digest, err := fileDigest(executable.path)
	if err != nil {
		return fmt.Errorf("catalog: verify %s executable: %w", executable.kind, err)
	}
	if digest != executable.contentSHA256 {
		return fmt.Errorf("catalog: %s executable identity drift at %q", executable.kind, executable.path)
	}
	return nil
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
