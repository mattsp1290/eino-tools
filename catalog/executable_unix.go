//go:build unix

package catalog

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/mattsp1290/eino-tools/search"
	"github.com/mattsp1290/eino-tools/shell"
)

type capturedExecutable struct {
	kind           string
	invocationPath string
	targetPath     string
	contentSHA256  string
	environment    string
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

	environment, executable, err := captureExecutable("ripgrep", options.RGBinary, options.Env)
	if err != nil {
		return search.Options{}, capturedExecutable{}, fmt.Errorf("catalog: search executable: %w", err)
	}
	options.RGBinary = executable.invocationPath
	options.Env = environment
	options.DisableConfig = true
	return options, executable, nil
}

func captureShellOptions(input *shell.Options) (shell.Options, capturedExecutable, error) {
	options := shell.Options{}
	if input != nil {
		options = *input
		options.Env = slices.Clone(input.Env)
	}
	if err := options.Validate(); err != nil {
		return shell.Options{}, capturedExecutable{}, fmt.Errorf("catalog: %w", err)
	}
	if options.StartupMode == "" {
		options.StartupMode = shell.StartupModeLogin
	}
	if options.OutputCapBytes == 0 {
		options.OutputCapBytes = shell.DefaultOutputCapBytes
	}
	if options.ShellBinary == "" {
		options.ShellBinary = shell.DefaultShellBinary
	}

	environment, executable, err := captureExecutable("shell", options.ShellBinary, options.Env)
	if err != nil {
		return shell.Options{}, capturedExecutable{}, fmt.Errorf("catalog: shell executable: %w", err)
	}
	options.ShellBinary = executable.invocationPath
	options.Env = environment
	return options, executable, nil
}

func captureExecutable(kind, binary string, inputEnvironment []string) ([]string, capturedExecutable, error) {
	environment := inputEnvironment
	if environment == nil {
		environment = os.Environ()
	}
	normalized, err := normalizeEnvironment(environment)
	if err != nil {
		return nil, capturedExecutable{}, fmt.Errorf("environment: %w", err)
	}
	invocationPath, targetPath, digest, err := resolveExecutable(binary, normalized)
	if err != nil {
		return nil, capturedExecutable{}, err
	}
	environmentDigest, err := hashJSON(normalized)
	if err != nil {
		return nil, capturedExecutable{}, fmt.Errorf("environment identity: %w", err)
	}
	return normalized, capturedExecutable{
		kind: kind, invocationPath: invocationPath, targetPath: targetPath,
		contentSHA256: digest, environment: environmentDigest,
	}, nil
}

func normalizeEnvironment(entries []string) ([]string, error) {
	values := make(map[string]string, len(entries))
	for _, entry := range entries {
		if strings.ContainsRune(entry, 0) {
			return nil, errors.New("environment entry contains NUL byte")
		}
		if !utf8.ValidString(entry) {
			return nil, errors.New("environment entry is not valid UTF-8")
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

func resolveExecutable(name string, environment []string) (string, string, string, error) {
	if strings.ContainsRune(name, 0) || name == "" {
		return "", "", "", errors.New("executable name is empty or contains NUL")
	}
	if !utf8.ValidString(name) {
		return "", "", "", errors.New("executable name is not valid UTF-8")
	}
	var candidates []string
	if filepath.IsAbs(name) {
		candidates = []string{name}
	} else {
		if strings.ContainsRune(name, filepath.Separator) {
			return "", "", "", fmt.Errorf("relative executable path %q contains a separator", name)
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
		target, digest, err := inspectExecutable(candidate)
		if err != nil {
			continue
		}
		return candidate, target, digest, nil
	}
	return "", "", "", fmt.Errorf("executable %q was not found as a readable regular executable", name)
}

func inspectExecutable(invocationPath string) (string, string, error) {
	if !utf8.ValidString(invocationPath) {
		return "", "", errors.New("executable invocation path is not valid UTF-8")
	}
	targetPath, err := filepath.EvalSymlinks(invocationPath)
	if err != nil {
		return "", "", err
	}
	if !utf8.ValidString(targetPath) {
		return "", "", errors.New("executable target path is not valid UTF-8")
	}
	info, err := os.Stat(targetPath)
	if err != nil {
		return "", "", err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
		return "", "", errors.New("executable target is not a regular executable")
	}
	digest, err := fileDigest(targetPath)
	if err != nil {
		return "", "", err
	}
	return targetPath, digest, nil
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

func (executable capturedExecutable) dependencies() []executorDependency {
	return []executorDependency{
		{Kind: executable.kind + "-invocation", Path: executable.invocationPath, ContentSHA256: executable.contentSHA256},
		{Kind: executable.kind + "-target", Path: executable.targetPath, ContentSHA256: executable.contentSHA256},
	}
}

func verifyExecutable(executable capturedExecutable) error {
	targetPath, digest, err := inspectExecutable(executable.invocationPath)
	if err != nil {
		return fmt.Errorf("catalog: verify %s executable: %w", executable.kind, err)
	}
	if targetPath != executable.targetPath || digest != executable.contentSHA256 {
		return fmt.Errorf("catalog: %s executable identity drift at %q", executable.kind, executable.invocationPath)
	}
	return nil
}
