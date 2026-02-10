// Package chassis provides the Chassis type for managing platform chassis structure.
// The chassis defines the skeleton of the platform - paths where nodes and components attach.
package chassis

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Chassis represents the platform chassis configuration.
// It preserves YAML order for consistent output.
type Chassis struct {
	node *yaml.Node
	data map[string]map[string][]interface{}
}

// Load reads and parses chassis.yaml from the given directory.
func Load(dir string) (*Chassis, error) {
	path := filepath.Join(dir, "chassis.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read chassis.yaml: %w", err)
	}

	var node yaml.Node
	if err := yaml.Unmarshal(data, &node); err != nil {
		return nil, fmt.Errorf("failed to parse chassis.yaml: %w", err)
	}

	var parsed map[string]map[string][]interface{}
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse chassis.yaml: %w", err)
	}

	return &Chassis{
		node: &node,
		data: parsed,
	}, nil
}

// Flatten returns all chassis paths in tree traversal order.
// Example output: ["platform", "platform.foundation", "platform.foundation.cluster", ...]
func (c *Chassis) Flatten() []string {
	if c.node == nil || len(c.node.Content) == 0 {
		return nil
	}

	var paths []string
	rootNode := c.node.Content[0]
	if rootNode.Kind != yaml.MappingNode {
		return nil
	}

	// Iterate root keys (e.g., "platform")
	for i := 0; i < len(rootNode.Content); i += 2 {
		rootKey := rootNode.Content[i].Value
		rootValue := rootNode.Content[i+1]
		paths = append(paths, rootKey)

		if rootValue.Kind != yaml.MappingNode {
			continue
		}

		// Iterate layers (e.g., "foundation", "interaction")
		for j := 0; j < len(rootValue.Content); j += 2 {
			layerKey := rootValue.Content[j].Value
			layerValue := rootValue.Content[j+1]
			layerPrefix := rootKey + "." + layerKey
			paths = append(paths, layerPrefix)

			if layerValue.Kind == yaml.SequenceNode {
				paths = append(paths, flattenSequence(layerPrefix, layerValue)...)
			}
		}
	}

	return paths
}

// flattenSequence recursively flattens a YAML sequence preserving order
func flattenSequence(prefix string, node *yaml.Node) []string {
	var paths []string

	for _, item := range node.Content {
		switch item.Kind {
		case yaml.ScalarNode:
			paths = append(paths, prefix+"."+item.Value)
		case yaml.MappingNode:
			for k := 0; k < len(item.Content); k += 2 {
				key := item.Content[k].Value
				value := item.Content[k+1]
				newPrefix := prefix + "." + key
				paths = append(paths, newPrefix)
				if value.Kind == yaml.SequenceNode {
					paths = append(paths, flattenSequence(newPrefix, value)...)
				}
			}
		}
	}

	return paths
}

// Exists checks if a chassis path exists.
func (c *Chassis) Exists(chassisPath string) bool {
	for _, path := range c.Flatten() {
		if path == chassisPath {
			return true
		}
	}
	return false
}

// Root returns the root chassis name (e.g., "platform").
func (c *Chassis) Root() string {
	paths := c.Flatten()
	if len(paths) > 0 {
		return paths[0]
	}
	return ""
}

// Children returns the direct children of a chassis path.
func (c *Chassis) Children(chassisPath string) []string {
	var children []string
	prefix := chassisPath + "."

	for _, path := range c.Flatten() {
		if strings.HasPrefix(path, prefix) {
			// Check it's a direct child (no more dots after prefix)
			remainder := path[len(prefix):]
			if !strings.Contains(remainder, ".") {
				children = append(children, path)
			}
		}
	}

	return children
}

// ChildrenMap returns a map of chassis path to its direct children.
func (c *Chassis) ChildrenMap() map[string][]string {
	result := make(map[string][]string)

	for _, chassisPath := range c.Flatten() {
		parent := Parent(chassisPath)
		if parent != "" {
			result[parent] = append(result[parent], chassisPath)
		}
	}

	return result
}

// Ancestors returns all ancestors of a given chassis path.
// Example: "platform.foundation.cluster.control" returns
// ["platform.foundation.cluster", "platform.foundation", "platform"]
func (c *Chassis) Ancestors(chassisPath string) []string {
	var ancestors []string
	current := chassisPath

	for {
		parent := Parent(current)
		if parent == "" {
			break
		}
		ancestors = append(ancestors, parent)
		current = parent
	}

	return ancestors
}

// AncestorsMap returns a map of chassis path to its ancestors.
func (c *Chassis) AncestorsMap() map[string][]string {
	result := make(map[string][]string)

	for _, chassisPath := range c.Flatten() {
		result[chassisPath] = c.Ancestors(chassisPath)
	}

	return result
}

// Parent returns the parent of a given chassis path.
// Example: "platform.foundation.cluster" returns "platform.foundation"
// Returns empty string for root paths.
func Parent(chassisPath string) string {
	idx := strings.LastIndex(chassisPath, ".")
	if idx == -1 {
		return ""
	}
	return chassisPath[:idx]
}

// IsDescendantOf checks if chassisPath is a descendant of ancestor.
func IsDescendantOf(chassisPath, ancestor string) bool {
	return strings.HasPrefix(chassisPath, ancestor+".")
}

// FlattenWithPrefix returns chassis paths that start with the given prefix.
func (c *Chassis) FlattenWithPrefix(prefix string) []string {
	all := c.Flatten()
	if prefix == "" {
		return all
	}

	var filtered []string
	for _, path := range all {
		if path == prefix || strings.HasPrefix(path, prefix+".") {
			filtered = append(filtered, path)
		}
	}
	return filtered
}
