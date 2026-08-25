package catalog

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/cloudwego/eino/schema"
)

const (
	schemaIdentityVersion   = "eino-tools-tool-schema-v1"
	executorIdentityVersion = "eino-tools-tool-executor-v1"
)

type schemaIdentity struct {
	Version     string `json:"version"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Parameters  any    `json:"parameters"`
}

type executorDependency struct {
	Kind          string `json:"kind"`
	Path          string `json:"path"`
	ContentSHA256 string `json:"content_sha256"`
}

type executorIdentity struct {
	Version        string               `json:"version"`
	RegistrationID string               `json:"registration_id"`
	Revision       int                  `json:"revision"`
	Dependencies   []executorDependency `json:"dependencies"`
	Environment    string               `json:"environment"`
}

func schemaHash(info *schema.ToolInfo) (string, error) {
	if info == nil || info.ParamsOneOf == nil {
		return "", fmt.Errorf("catalog: tool metadata has no parameter schema")
	}
	parameters, err := info.ToJSONSchema()
	if err != nil {
		return "", fmt.Errorf("catalog: convert parameter schema: %w", err)
	}
	return hashJSON(schemaIdentity{
		Version:     schemaIdentityVersion,
		Name:        info.Name,
		Description: info.Desc,
		Parameters:  parameters,
	})
}

func executorHash(id string, revision int, dependencies []executorDependency, environment string) (string, error) {
	deps := append([]executorDependency(nil), dependencies...)
	sort.Slice(deps, func(i, j int) bool {
		if deps[i].Kind == deps[j].Kind {
			return deps[i].Path < deps[j].Path
		}
		return deps[i].Kind < deps[j].Kind
	})
	if deps == nil {
		deps = []executorDependency{}
	}
	return hashJSON(executorIdentity{
		Version:        executorIdentityVersion,
		RegistrationID: id,
		Revision:       revision,
		Dependencies:   deps,
		Environment:    environment,
	})
}

func hashJSON(value interface{}) (string, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("catalog: marshal identity: %w", err)
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}
