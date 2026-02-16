// Package chassis provides shared logic for chassis operations
package chassis

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	pkgchassis "github.com/plasmash/plasmactl-chassis/pkg/chassis"
)

// Chassis wraps the public Chassis type with write operations.
type Chassis struct {
	*pkgchassis.Chassis
}

// Node represents a node file from inst/<platform>/nodes/<hostname>.yaml
type Node struct {
	Hostname string   `yaml:"hostname"`
	Chassis  []string `yaml:"chassis"`
}

// Load reads and parses chassis.yaml from the given directory
func Load(dir string) (*Chassis, error) {
	pub, err := pkgchassis.Load(dir)
	if err != nil {
		return nil, err
	}
	return &Chassis{Chassis: pub}, nil
}

// Save writes the chassis configuration to chassis.yaml preserving order
func (c *Chassis) Save(dir string) error {
	path := filepath.Join(dir, "chassis.yaml")
	data, err := yaml.Marshal(c.YAMLNode())
	if err != nil {
		return fmt.Errorf("failed to marshal chassis: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

// Add adds a new chassis path preserving YAML order
// Path format: any dotted path (e.g., platform, platform.bite, platform.foundation.cluster)
func (c *Chassis) Add(chassisPath string) error {
	if err := pkgchassis.ValidatePath(chassisPath); err != nil {
		return err
	}

	parts := strings.Split(chassisPath, ".")

	if c.Exists(chassisPath) {
		return fmt.Errorf("chassis path %q already exists", chassisPath)
	}

	// Work with yaml.Node to preserve order
	node := c.YAMLNode()
	if node == nil || len(node.Content) == 0 {
		// Create new document node
		newNode := &yaml.Node{
			Kind: yaml.DocumentNode,
			Content: []*yaml.Node{{
				Kind: yaml.MappingNode,
			}},
		}
		c.SetYAMLNode(newNode)
		node = newNode
	}

	rootNode := node.Content[0]

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

	// Also update data for consistency
	d := c.RawData()
	if d == nil {
		d = make(map[string]map[string][]interface{})
		c.SetRawData(d)
	}
	if len(parts) >= 2 {
		root := parts[0]
		layer := parts[1]
		if d[root] == nil {
			d[root] = make(map[string][]interface{})
		}
		if len(parts) > 2 {
			d[root][layer] = addChassisPath(d[root][layer], parts[2:])
		} else {
			// Just ensure the layer exists
			if d[root][layer] == nil {
				d[root][layer] = []interface{}{}
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

// Remove removes a chassis path preserving YAML order
func (c *Chassis) Remove(chassisPath string) error {
	parts := strings.Split(chassisPath, ".")
	if len(parts) < 1 || chassisPath == "" {
		return fmt.Errorf("chassis path cannot be empty")
	}

	if !c.Exists(chassisPath) {
		return fmt.Errorf("chassis path %q does not exist", chassisPath)
	}

	// Remove from yaml.Node
	node := c.YAMLNode()
	if node != nil && len(node.Content) > 0 {
		rootNode := node.Content[0]
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

	// Also update data for consistency
	d := c.RawData()
	if d == nil {
		return nil
	}

	if len(parts) == 1 {
		delete(d, parts[0])
		return nil
	}

	root := parts[0]
	layer := parts[1]

	if len(parts) == 2 {
		if d[root] != nil {
			delete(d[root], layer)
		}
		return nil
	}

	remaining := parts[2:]
	var removed bool
	d[root][layer], removed = removeChassisPath(d[root][layer], remaining)
	if !removed {
		return fmt.Errorf("failed to remove chassis path %q", chassisPath)
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
	d := c.RawData()
	for root, layers := range d {
		for layer, chassis := range layers {
			tree[root+"."+layer] = chassisToTree(chassis)
		}
	}
	return tree
}

// addChassisPath adds a chassis path to the nested structure
func addChassisPath(chassis []interface{}, path []string) []interface{} {
	if len(path) == 0 {
		return chassis
	}

	name := path[0]
	remaining := path[1:]

	// If this is the last segment, add as string
	if len(remaining) == 0 {
		// Check if it already exists
		for _, c := range chassis {
			if str, ok := c.(string); ok && str == name {
				return chassis
			}
		}
		return append(chassis, name)
	}

	// Need to add nested structure
	for i, c := range chassis {
		if m, ok := c.(map[string]interface{}); ok {
			if sub, exists := m[name]; exists {
				if subSlice, ok := sub.([]interface{}); ok {
					m[name] = addChassisPath(subSlice, remaining)
					return chassis
				}
			}
		}
		if str, ok := c.(string); ok && str == name {
			// Convert string to map with nested content
			chassis[i] = map[string]interface{}{
				name: addChassisPath(nil, remaining),
			}
			return chassis
		}
	}

	// Create new nested structure
	newMap := map[string]interface{}{
		name: addChassisPath(nil, remaining),
	}
	return append(chassis, newMap)
}

// removeChassisPath removes a chassis path from the nested structure
func removeChassisPath(chassis []interface{}, path []string) ([]interface{}, bool) {
	if len(path) == 0 {
		return chassis, false
	}

	name := path[0]
	remaining := path[1:]

	for i, c := range chassis {
		// Check string match
		if str, ok := c.(string); ok && str == name && len(remaining) == 0 {
			return append(chassis[:i], chassis[i+1:]...), true
		}

		// Check map match
		if m, ok := c.(map[string]interface{}); ok {
			if sub, exists := m[name]; exists {
				if len(remaining) == 0 {
					// Remove the entire map entry
					delete(m, name)
					if len(m) == 0 {
						return append(chassis[:i], chassis[i+1:]...), true
					}
					return chassis, true
				}
				if subSlice, ok := sub.([]interface{}); ok {
					newSub, removed := removeChassisPath(subSlice, remaining)
					if removed {
						m[name] = newSub
						return chassis, true
					}
				}
			}
		}
	}

	return chassis, false
}

// chassisToTree converts chassis structure to a displayable tree
func chassisToTree(chassis []interface{}) interface{} {
	if len(chassis) == 0 {
		return nil
	}

	result := make(map[string]interface{})
	for _, c := range chassis {
		switch item := c.(type) {
		case string:
			result[item] = nil
		case map[string]interface{}:
			for name, sub := range item {
				if subSlice, ok := sub.([]interface{}); ok {
					result[name] = chassisToTree(subSlice)
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

// NodesForChassis returns nodes allocated to a chassis path or its children
func NodesForChassis(nodes []Node, chassisPath string) []Node {
	var result []Node
	for _, node := range nodes {
		for _, c := range node.Chassis {
			// Match exact chassis path or children
			if c == chassisPath || strings.HasPrefix(c, chassisPath+".") {
				result = append(result, node)
				break
			}
		}
	}
	return result
}

// LoadNodesByPlatform groups nodes by their platform
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

// Rename renames a chassis path preserving YAML order
func (c *Chassis) Rename(oldPath, newPath string) error {
	oldParts := strings.Split(oldPath, ".")
	newParts := strings.Split(newPath, ".")

	if len(oldParts) != len(newParts) {
		return fmt.Errorf("old and new paths must have the same depth")
	}

	// Find the differing segment (should be exactly one for a rename)
	diffIdx := -1
	for i := 0; i < len(oldParts); i++ {
		if oldParts[i] != newParts[i] {
			if diffIdx != -1 {
				return fmt.Errorf("rename can only change one segment at a time")
			}
			diffIdx = i
		}
	}

	if diffIdx == -1 {
		return fmt.Errorf("old and new paths are identical")
	}

	// Update yaml.Node
	node := c.YAMLNode()
	if node != nil && len(node.Content) > 0 {
		renameInNode(node.Content[0], oldParts, newParts, diffIdx, 0)
	}

	// Update data for consistency
	c.updateDataForRename(oldParts, newParts, diffIdx)

	return nil
}

// renameInNode recursively finds and renames the target segment in yaml.Node
func renameInNode(node *yaml.Node, oldParts, newParts []string, diffIdx, depth int) bool {
	if node == nil || depth >= len(oldParts) {
		return false
	}

	target := oldParts[depth]
	newName := newParts[depth]

	switch node.Kind {
	case yaml.MappingNode:
		for i := 0; i < len(node.Content); i += 2 {
			key := node.Content[i]
			value := node.Content[i+1]

			if key.Value == target {
				if depth == diffIdx {
					// This is the segment to rename
					key.Value = newName
					return true
				}
				// Continue deeper
				return renameInNode(value, oldParts, newParts, diffIdx, depth+1)
			}
		}
	case yaml.SequenceNode:
		for _, item := range node.Content {
			if item.Kind == yaml.ScalarNode && item.Value == target {
				if depth == diffIdx {
					item.Value = newName
					return true
				}
			} else if item.Kind == yaml.MappingNode {
				for j := 0; j < len(item.Content); j += 2 {
					if item.Content[j].Value == target {
						if depth == diffIdx {
							item.Content[j].Value = newName
							return true
						}
						return renameInNode(item.Content[j+1], oldParts, newParts, diffIdx, depth+1)
					}
				}
			}
		}
	}

	return false
}

// updateDataForRename updates data after a rename
func (c *Chassis) updateDataForRename(oldParts, newParts []string, diffIdx int) {
	d := c.RawData()
	if d == nil {
		return
	}

	switch diffIdx {
	case 0:
		// Renaming root key
		if data, exists := d[oldParts[0]]; exists {
			d[newParts[0]] = data
			delete(d, oldParts[0])
		}
	case 1:
		// Renaming layer key
		root := oldParts[0]
		if d[root] != nil {
			if data, exists := d[root][oldParts[1]]; exists {
				d[root][newParts[1]] = data
				delete(d[root], oldParts[1])
			}
		}
	default:
		// Renaming within nested chassis structure
		root := oldParts[0]
		layer := oldParts[1]
		if d[root] != nil && d[root][layer] != nil {
			d[root][layer] = renameInChassisData(d[root][layer], oldParts[2:], newParts[2:], diffIdx-2)
		}
	}
}

// renameInChassisData renames a path segment within the chassis data structure
func renameInChassisData(chassis []interface{}, oldPath, newPath []string, diffIdx int) []interface{} {
	if len(oldPath) == 0 || diffIdx < 0 {
		return chassis
	}

	target := oldPath[0]
	newName := newPath[0]

	for i, item := range chassis {
		switch v := item.(type) {
		case string:
			if v == target && diffIdx == 0 {
				// Rename this string entry
				chassis[i] = newName
				return chassis
			}
		case map[string]interface{}:
			if sub, exists := v[target]; exists {
				if diffIdx == 0 {
					// Rename the key in this map
					v[newName] = sub
					delete(v, target)
					return chassis
				}
				// Recurse deeper
				if subSlice, ok := sub.([]interface{}); ok {
					v[target] = renameInChassisData(subSlice, oldPath[1:], newPath[1:], diffIdx-1)
					return chassis
				}
			}
		}
	}

	return chassis
}
