// Package chassis provides shared logic for chassis operations
package chassis

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// OrderedChassis represents the platform chassis configuration preserving YAML order
type OrderedChassis struct {
	node *yaml.Node
	data map[string]map[string][]interface{}
}

// Chassis is an alias for backward compatibility
type Chassis = OrderedChassis

// Node represents a node file from inst/<platform>/nodes/<hostname>.yaml
type Node struct {
	Hostname string   `yaml:"hostname"`
	Chassis  []string `yaml:"chassis"`
}

// Load reads and parses chassis.yaml from the given directory
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

// Save writes the chassis configuration to chassis.yaml preserving order
func (c *Chassis) Save(dir string) error {
	path := filepath.Join(dir, "chassis.yaml")
	data, err := yaml.Marshal(c.node)
	if err != nil {
		return fmt.Errorf("failed to marshal chassis: %w", err)
	}

	return os.WriteFile(path, data, 0644)
}

// Flatten returns all chassis section paths preserving YAML order
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
			// Simple string item
			paths = append(paths, prefix+"."+item.Value)
		case yaml.MappingNode:
			// Nested structure like "cluster: [control, nodes]"
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

// FlattenWithPrefix returns chassis paths that start with the given prefix
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

// Exists checks if a section path exists in the chassis
func (c *Chassis) Exists(section string) bool {
	for _, path := range c.Flatten() {
		if path == section {
			return true
		}
	}
	return false
}

// Add adds a new section to the chassis preserving YAML order
// Section format: any dotted path (e.g., platform, platform.bite, platform.foundation.cluster)
func (c *Chassis) Add(section string) error {
	parts := strings.Split(section, ".")
	if len(parts) < 1 || section == "" {
		return fmt.Errorf("section cannot be empty")
	}

	if c.Exists(section) {
		return fmt.Errorf("section %q already exists", section)
	}

	// Work with yaml.Node to preserve order
	if c.node == nil || len(c.node.Content) == 0 {
		// Create new document node
		c.node = &yaml.Node{
			Kind: yaml.DocumentNode,
			Content: []*yaml.Node{{
				Kind: yaml.MappingNode,
			}},
		}
	}

	rootNode := c.node.Content[0]

	if len(parts) == 1 {
		// Just a root key (e.g., "platform")
		findOrCreateMapKey(rootNode, parts[0])
	} else if len(parts) == 2 {
		// Root and layer (e.g., "platform.bite")
		root := parts[0]
		layer := parts[1]
		rootValueNode := findOrCreateMapKey(rootNode, root)
		layerValueNode := findOrCreateMapKey(rootValueNode, layer)
		// Ensure it's a sequence node (empty)
		if layerValueNode.Kind != yaml.SequenceNode {
			layerValueNode.Kind = yaml.SequenceNode
			layerValueNode.Content = nil
		}
	} else {
		// Full path (e.g., "platform.foundation.cluster")
		root := parts[0]
		layer := parts[1]
		remaining := parts[2:]

		rootValueNode := findOrCreateMapKey(rootNode, root)
		layerValueNode := findOrCreateMapKey(rootValueNode, layer)

		// Ensure it's a sequence node
		if layerValueNode.Kind != yaml.SequenceNode {
			layerValueNode.Kind = yaml.SequenceNode
			layerValueNode.Content = nil
		}

		// Add the remaining path to the sequence
		addPathToSequence(layerValueNode, remaining)
	}

	// Also update c.data for consistency
	if c.data == nil {
		c.data = make(map[string]map[string][]interface{})
	}
	if len(parts) >= 2 {
		root := parts[0]
		layer := parts[1]
		if c.data[root] == nil {
			c.data[root] = make(map[string][]interface{})
		}
		if len(parts) > 2 {
			c.data[root][layer] = addToSections(c.data[root][layer], parts[2:])
		} else {
			// Just ensure the layer exists
			if c.data[root][layer] == nil {
				c.data[root][layer] = []interface{}{}
			}
		}
	}

	return nil
}

// findOrCreateMapKey finds a key in a mapping node or creates it at the end
func findOrCreateMapKey(mapNode *yaml.Node, key string) *yaml.Node {
	if mapNode.Kind != yaml.MappingNode {
		mapNode.Kind = yaml.MappingNode
		mapNode.Content = nil
	}

	// Look for existing key
	for i := 0; i < len(mapNode.Content); i += 2 {
		if mapNode.Content[i].Value == key {
			return mapNode.Content[i+1]
		}
	}

	// Key not found, create at end
	keyNode := &yaml.Node{
		Kind:  yaml.ScalarNode,
		Tag:   "!!str",
		Value: key,
	}
	valueNode := &yaml.Node{
		Kind: yaml.MappingNode,
	}
	mapNode.Content = append(mapNode.Content, keyNode, valueNode)
	return valueNode
}

// addPathToSequence adds a dotted path to a sequence node
func addPathToSequence(seqNode *yaml.Node, path []string) {
	if len(path) == 0 {
		return
	}

	name := path[0]
	remaining := path[1:]

	if len(remaining) == 0 {
		// Last segment - add as scalar
		// Check if it already exists
		for _, item := range seqNode.Content {
			if item.Kind == yaml.ScalarNode && item.Value == name {
				return // Already exists
			}
		}
		// Add new scalar at end
		seqNode.Content = append(seqNode.Content, &yaml.Node{
			Kind:  yaml.ScalarNode,
			Tag:   "!!str",
			Value: name,
		})
		return
	}

	// Need nested structure - look for existing map with this key
	for _, item := range seqNode.Content {
		if item.Kind == yaml.MappingNode {
			for i := 0; i < len(item.Content); i += 2 {
				if item.Content[i].Value == name {
					// Found existing key, recurse into its value
					valueNode := item.Content[i+1]
					if valueNode.Kind != yaml.SequenceNode {
						valueNode.Kind = yaml.SequenceNode
						valueNode.Content = nil
					}
					addPathToSequence(valueNode, remaining)
					return
				}
			}
		}
	}

	// Check if name exists as a scalar and convert it
	for i, item := range seqNode.Content {
		if item.Kind == yaml.ScalarNode && item.Value == name {
			// Convert scalar to map with sequence
			newSeq := &yaml.Node{Kind: yaml.SequenceNode}
			addPathToSequence(newSeq, remaining)
			seqNode.Content[i] = &yaml.Node{
				Kind: yaml.MappingNode,
				Content: []*yaml.Node{
					{Kind: yaml.ScalarNode, Tag: "!!str", Value: name},
					newSeq,
				},
			}
			return
		}
	}

	// Create new map entry at end of sequence
	newSeq := &yaml.Node{Kind: yaml.SequenceNode}
	addPathToSequence(newSeq, remaining)
	seqNode.Content = append(seqNode.Content, &yaml.Node{
		Kind: yaml.MappingNode,
		Content: []*yaml.Node{
			{Kind: yaml.ScalarNode, Tag: "!!str", Value: name},
			newSeq,
		},
	})
}

// Remove removes a section from the chassis preserving YAML order
func (c *Chassis) Remove(section string) error {
	parts := strings.Split(section, ".")
	if len(parts) < 1 || section == "" {
		return fmt.Errorf("section cannot be empty")
	}

	if !c.Exists(section) {
		return fmt.Errorf("section %q does not exist", section)
	}

	// Remove from yaml.Node
	if c.node != nil && len(c.node.Content) > 0 {
		rootNode := c.node.Content[0]
		if rootNode.Kind == yaml.MappingNode {
			if len(parts) == 1 {
				// Remove root key entirely
				root := parts[0]
				for i := 0; i < len(rootNode.Content); i += 2 {
					if rootNode.Content[i].Value == root {
						rootNode.Content = append(rootNode.Content[:i], rootNode.Content[i+2:]...)
						break
					}
				}
			} else if len(parts) == 2 {
				// Remove layer from root
				root := parts[0]
				layer := parts[1]
				for i := 0; i < len(rootNode.Content); i += 2 {
					if rootNode.Content[i].Value == root {
						rootValueNode := rootNode.Content[i+1]
						if rootValueNode.Kind == yaml.MappingNode {
							for j := 0; j < len(rootValueNode.Content); j += 2 {
								if rootValueNode.Content[j].Value == layer {
									rootValueNode.Content = append(rootValueNode.Content[:j], rootValueNode.Content[j+2:]...)
									break
								}
							}
						}
						break
					}
				}
			} else {
				// Remove from nested structure
				root := parts[0]
				layer := parts[1]
				remaining := parts[2:]
				for i := 0; i < len(rootNode.Content); i += 2 {
					if rootNode.Content[i].Value == root {
						rootValueNode := rootNode.Content[i+1]
						if rootValueNode.Kind == yaml.MappingNode {
							for j := 0; j < len(rootValueNode.Content); j += 2 {
								if rootValueNode.Content[j].Value == layer {
									layerValueNode := rootValueNode.Content[j+1]
									if layerValueNode.Kind == yaml.SequenceNode {
										removePathFromSequence(layerValueNode, remaining)
									}
									break
								}
							}
						}
						break
					}
				}
			}
		}
	}

	// Also update c.data for consistency
	if len(parts) == 1 {
		delete(c.data, parts[0])
		return nil
	}

	root := parts[0]
	layer := parts[1]

	if len(parts) == 2 {
		if c.data[root] != nil {
			delete(c.data[root], layer)
		}
		return nil
	}

	remaining := parts[2:]
	var removed bool
	c.data[root][layer], removed = removeFromSections(c.data[root][layer], remaining)
	if !removed {
		return fmt.Errorf("failed to remove section %q", section)
	}

	return nil
}

// removePathFromSequence removes a dotted path from a sequence node
func removePathFromSequence(seqNode *yaml.Node, path []string) bool {
	if len(path) == 0 {
		return false
	}

	name := path[0]
	remaining := path[1:]

	for i, item := range seqNode.Content {
		if len(remaining) == 0 {
			// Looking for exact match to remove
			if item.Kind == yaml.ScalarNode && item.Value == name {
				seqNode.Content = append(seqNode.Content[:i], seqNode.Content[i+1:]...)
				return true
			}
			if item.Kind == yaml.MappingNode {
				for j := 0; j < len(item.Content); j += 2 {
					if item.Content[j].Value == name {
						// Remove entire map entry or just the key
						if len(item.Content) == 2 {
							// Only this key in map, remove the whole map item
							seqNode.Content = append(seqNode.Content[:i], seqNode.Content[i+1:]...)
						} else {
							// Multiple keys, just remove this one
							item.Content = append(item.Content[:j], item.Content[j+2:]...)
						}
						return true
					}
				}
			}
		} else {
			// Need to recurse
			if item.Kind == yaml.MappingNode {
				for j := 0; j < len(item.Content); j += 2 {
					if item.Content[j].Value == name {
						valueNode := item.Content[j+1]
						if valueNode.Kind == yaml.SequenceNode {
							return removePathFromSequence(valueNode, remaining)
						}
					}
				}
			}
		}
	}

	return false
}

// GetTree returns the chassis as a tree structure for display
func (c *Chassis) GetTree() map[string]interface{} {
	tree := make(map[string]interface{})
	for root, layers := range c.data {
		for layer, sections := range layers {
			tree[root+"."+layer] = sectionsToTree(sections)
		}
	}
	return tree
}

// addToSections adds a path to the sections structure
func addToSections(sections []interface{}, path []string) []interface{} {
	if len(path) == 0 {
		return sections
	}

	name := path[0]
	remaining := path[1:]

	// If this is the last segment, add as string
	if len(remaining) == 0 {
		// Check if it already exists
		for _, s := range sections {
			if str, ok := s.(string); ok && str == name {
				return sections
			}
		}
		return append(sections, name)
	}

	// Need to add nested structure
	for i, s := range sections {
		if m, ok := s.(map[string]interface{}); ok {
			if sub, exists := m[name]; exists {
				if subSlice, ok := sub.([]interface{}); ok {
					m[name] = addToSections(subSlice, remaining)
					return sections
				}
			}
		}
		if str, ok := s.(string); ok && str == name {
			// Convert string to map with nested content
			sections[i] = map[string]interface{}{
				name: addToSections(nil, remaining),
			}
			return sections
		}
	}

	// Create new nested structure
	newMap := map[string]interface{}{
		name: addToSections(nil, remaining),
	}
	return append(sections, newMap)
}

// removeFromSections removes a path from the sections structure
func removeFromSections(sections []interface{}, path []string) ([]interface{}, bool) {
	if len(path) == 0 {
		return sections, false
	}

	name := path[0]
	remaining := path[1:]

	for i, s := range sections {
		// Check string match
		if str, ok := s.(string); ok && str == name && len(remaining) == 0 {
			return append(sections[:i], sections[i+1:]...), true
		}

		// Check map match
		if m, ok := s.(map[string]interface{}); ok {
			if sub, exists := m[name]; exists {
				if len(remaining) == 0 {
					// Remove the entire map entry
					delete(m, name)
					if len(m) == 0 {
						return append(sections[:i], sections[i+1:]...), true
					}
					return sections, true
				}
				if subSlice, ok := sub.([]interface{}); ok {
					newSub, removed := removeFromSections(subSlice, remaining)
					if removed {
						m[name] = newSub
						return sections, true
					}
				}
			}
		}
	}

	return sections, false
}

// sectionsToTree converts sections to a displayable tree
func sectionsToTree(sections []interface{}) interface{} {
	if len(sections) == 0 {
		return nil
	}

	result := make(map[string]interface{})
	for _, s := range sections {
		switch section := s.(type) {
		case string:
			result[section] = nil
		case map[string]interface{}:
			for name, sub := range section {
				if subSlice, ok := sub.([]interface{}); ok {
					result[name] = sectionsToTree(subSlice)
				} else {
					result[name] = nil
				}
			}
		}
	}
	return result
}

// LoadNodes loads all nodes from inst/<platform>/nodes/ directory
func LoadNodes(dir, platform string) ([]Node, error) {
	var nodes []Node

	instDir := filepath.Join(dir, "inst")
	if platform != "" {
		// Load from specific platform
		nodes, err := loadNodesFromPlatform(instDir, platform)
		if err != nil {
			return nil, err
		}
		return nodes, nil
	}

	// Load from all platforms
	entries, err := os.ReadDir(instDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read inst directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		platformNodes, err := loadNodesFromPlatform(instDir, entry.Name())
		if err != nil {
			continue // Skip platforms with errors
		}
		nodes = append(nodes, platformNodes...)
	}

	return nodes, nil
}

func loadNodesFromPlatform(instDir, platform string) ([]Node, error) {
	nodesDir := filepath.Join(instDir, platform, "nodes")
	entries, err := os.ReadDir(nodesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var nodes []Node
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(nodesDir, entry.Name()))
		if err != nil {
			continue
		}

		var node Node
		if err := yaml.Unmarshal(data, &node); err != nil {
			continue
		}
		node.Hostname = strings.TrimSuffix(entry.Name(), ".yaml")
		nodes = append(nodes, node)
	}

	return nodes, nil
}

// NodesForSection returns nodes allocated to a specific chassis section
func NodesForSection(nodes []Node, section string) []Node {
	var result []Node
	for _, node := range nodes {
		for _, c := range node.Chassis {
			if c == section {
				result = append(result, node)
				break
			}
		}
	}
	return result
}

// NodesByPlatform groups nodes by their platform
func LoadNodesByPlatform(dir string) (map[string][]Node, error) {
	result := make(map[string][]Node)

	instDir := filepath.Join(dir, "inst")
	entries, err := os.ReadDir(instDir)
	if err != nil {
		if os.IsNotExist(err) {
			return result, nil
		}
		return nil, fmt.Errorf("failed to read inst directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		nodes, err := loadNodesFromPlatform(instDir, entry.Name())
		if err != nil {
			continue
		}
		if len(nodes) > 0 {
			result[entry.Name()] = nodes
		}
	}

	return result, nil
}

// ComponentAttachment represents a component attached to a chassis section
type ComponentAttachment struct {
	Component string
	Playbook  string
}

// LoadAttachments scans playbooks for component attachments to a chassis section
func LoadAttachments(dir, section string) ([]ComponentAttachment, error) {
	var attachments []ComponentAttachment

	// Scan src/<layer>/<layer>.yaml playbooks
	srcDir := filepath.Join(dir, "src")
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		playbookPath := filepath.Join(srcDir, entry.Name(), entry.Name()+".yaml")
		data, err := os.ReadFile(playbookPath)
		if err != nil {
			continue
		}

		// Parse playbook
		var plays []struct {
			Hosts string   `yaml:"hosts"`
			Roles []string `yaml:"roles"`
		}
		if err := yaml.Unmarshal(data, &plays); err != nil {
			continue
		}

		for _, play := range plays {
			if play.Hosts == section {
				for _, role := range play.Roles {
					attachments = append(attachments, ComponentAttachment{
						Component: role,
						Playbook:  playbookPath,
					})
				}
			}
		}
	}

	return attachments, nil
}
