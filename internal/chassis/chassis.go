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

// Save writes the chassis configuration to chassis.yaml
func (c *Chassis) Save(dir string) error {
	path := filepath.Join(dir, "chassis.yaml")
	data, err := yaml.Marshal(c.data)
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

// Add adds a new section to the chassis
// Section format: root.layer.path.to.section (e.g., platform.foundation.cluster)
func (c *Chassis) Add(section string) error {
	parts := strings.Split(section, ".")
	if len(parts) < 3 {
		return fmt.Errorf("invalid section format: need at least root.layer.section")
	}

	if c.Exists(section) {
		return fmt.Errorf("section %q already exists", section)
	}

	root := parts[0]
	layer := parts[1]
	remaining := parts[2:]

	if c.data[root] == nil {
		c.data[root] = make(map[string][]interface{})
	}

	c.data[root][layer] = addToSections(c.data[root][layer], remaining)
	return nil
}

// Remove removes a section from the chassis
func (c *Chassis) Remove(section string) error {
	parts := strings.Split(section, ".")
	if len(parts) < 3 {
		return fmt.Errorf("invalid section format: need at least root.layer.section")
	}

	if !c.Exists(section) {
		return fmt.Errorf("section %q does not exist", section)
	}

	root := parts[0]
	layer := parts[1]
	remaining := parts[2:]

	var removed bool
	c.data[root][layer], removed = removeFromSections(c.data[root][layer], remaining)
	if !removed {
		return fmt.Errorf("failed to remove section %q", section)
	}

	return nil
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
