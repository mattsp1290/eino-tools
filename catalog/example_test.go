//go:build unix

package catalog_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/cloudwego/eino/components/tool"

	"github.com/mattsp1290/eino-tools/catalog"
)

type hostRegistration struct {
	id               string
	name             string
	binding          catalog.BindingKind
	mountScope       string
	retrySafe        bool
	concurrent       bool
	schemaIdentity   string
	executorIdentity string
	lockKey          func(canonicalRoot string) string
	newTool          func(context.Context) (tool.InvokableTool, error)
}

func composeIdentity(version string, inputs ...string) string {
	record := struct {
		Version string   `json:"version"`
		Inputs  []string `json:"inputs"`
	}{Version: version, Inputs: inputs}
	raw, err := json.Marshal(record)
	if err != nil {
		panic(err)
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func translate(definition catalog.Definition, canonicalRoot string) hostRegistration {
	// Metadata inspection is independent of a workspace instance.
	info, err := definition.Info()
	if err != nil {
		panic(err)
	}

	registration := hostRegistration{
		id:         definition.ID,
		name:       info.Name,
		binding:    definition.Binding,
		mountScope: "session", // Host mount scope is independent of leaf binding.
		retrySafe:  definition.RetrySafe,
		concurrent: definition.Concurrent,
		// The real host composes leaf identity with permissions, retention,
		// policy, artifact, and configuration identity before persistence.
		schemaIdentity: composeIdentity("host-tool-schema-v1",
			definition.SchemaHash, "host-permissions-and-retention-v3"),
		executorIdentity: composeIdentity("host-tool-executor-v1",
			definition.ExecutorHash, "host-artifact-sha256"),
		newTool: func(ctx context.Context) (tool.InvokableTool, error) {
			return definition.New(ctx, catalog.Instance{WorkspaceRoot: canonicalRoot})
		},
	}
	if definition.Concurrent {
		registration.lockKey = func(string) string { return "" }
	} else if definition.Binding == catalog.BindingWorkspace {
		registration.lockKey = func(root string) string { return "workspace:" + root }
	} else {
		registration.lockKey = func(string) string { return "registration:" + definition.ID }
	}

	// Keep every safety field live in the translated record. The real host
	// attaches artifact/config/permission/retention/locking policy and mounts
	// the complete translated set atomically.
	if registration.id != definition.ID || registration.name != definition.Name ||
		registration.binding != definition.Binding || registration.retrySafe != definition.RetrySafe ||
		registration.concurrent != definition.Concurrent || registration.lockKey == nil {
		panic("incomplete catalog translation")
	}
	return registration
}

func ExampleStandard() {
	definitions, err := catalog.Standard(catalog.Options{})
	if err != nil {
		panic(err)
	}
	canonicalRoot := "/host/admitted/canonical/workspace"
	registration := translate(definitions[0], canonicalRoot)
	fmt.Println(registration.id, registration.mountScope, registration.lockKey(canonicalRoot))
	// The host later calls registration.newTool only after retaining authority
	// over canonicalRoot for the frozen run-plan lifetime. Static URL fetch still
	// requires host network/filesystem permission policy.
	_ = registration.newTool

	// Output:
	// standard.file-read session workspace:/host/admitted/canonical/workspace
}
