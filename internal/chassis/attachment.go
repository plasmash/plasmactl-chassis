package chassis

import (
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Attachment represents a component attached to a chassis path
type Attachment struct {
	Component string
	Playbook  string
	Chassis   string
}

// LoadAttachments scans playbooks for component attachments to a chassis path
func LoadAttachments(dir, chassisPath string) ([]Attachment, error) {
	var attachments []Attachment

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

		// Parse playbook - roles can be strings or dicts with "role" key
		var plays []struct {
			Hosts string        `yaml:"hosts"`
			Roles []interface{} `yaml:"roles"`
		}
		if err := yaml.Unmarshal(data, &plays); err != nil {
			continue
		}

		for _, play := range plays {
			// Match exact chassis path or children
			if play.Hosts == chassisPath || strings.HasPrefix(play.Hosts, chassisPath+".") {
				for _, r := range play.Roles {
					var roleName string
					switch role := r.(type) {
					case string:
						// Simple string: "- foundation.applications.os"
						roleName = role
					case map[string]interface{}:
						// Dict with role key: "- role: foundation.applications.cluster"
						if name, ok := role["role"].(string); ok {
							roleName = name
						}
					}
					if roleName != "" {
						attachments = append(attachments, Attachment{
							Component: roleName,
							Playbook:  playbookPath,
							Chassis:   play.Hosts,
						})
					}
				}
			}
		}
	}

	return attachments, nil
}

// HasAttachments checks if a chassis path has any component attachments
func HasAttachments(dir, chassisPath string) (bool, []Attachment, error) {
	attachments, err := LoadAttachments(dir, chassisPath)
	if err != nil {
		return false, nil, err
	}
	return len(attachments) > 0, attachments, nil
}

// UpdateAttachments renames chassis path references in all playbooks
func UpdateAttachments(dir, oldChassis, newChassis string) ([]string, error) {
	var updatedFiles []string

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

		// Parse as yaml.Node to preserve formatting
		var doc yaml.Node
		if err := yaml.Unmarshal(data, &doc); err != nil {
			continue
		}

		updated := updateHostsInNode(&doc, oldChassis, newChassis)
		if updated {
			newData, err := yaml.Marshal(&doc)
			if err != nil {
				continue
			}
			if err := os.WriteFile(playbookPath, newData, 0644); err != nil {
				continue
			}
			updatedFiles = append(updatedFiles, playbookPath)
		}
	}

	return updatedFiles, nil
}

// updateHostsInNode recursively updates hosts fields in a yaml.Node
func updateHostsInNode(node *yaml.Node, oldChassis, newChassis string) bool {
	updated := false

	switch node.Kind {
	case yaml.DocumentNode:
		for _, child := range node.Content {
			if updateHostsInNode(child, oldChassis, newChassis) {
				updated = true
			}
		}
	case yaml.SequenceNode:
		for _, child := range node.Content {
			if updateHostsInNode(child, oldChassis, newChassis) {
				updated = true
			}
		}
	case yaml.MappingNode:
		for i := 0; i < len(node.Content); i += 2 {
			key := node.Content[i]
			value := node.Content[i+1]

			if key.Value == "hosts" && value.Kind == yaml.ScalarNode {
				// Check for exact match or prefix match
				if value.Value == oldChassis {
					value.Value = newChassis
					updated = true
				} else if strings.HasPrefix(value.Value, oldChassis+".") {
					value.Value = newChassis + value.Value[len(oldChassis):]
					updated = true
				}
			} else {
				if updateHostsInNode(value, oldChassis, newChassis) {
					updated = true
				}
			}
		}
	}

	return updated
}

// UpdateAllocations renames chassis path references in all node files
func UpdateAllocations(dir, oldChassis, newChassis string) ([]string, error) {
	var updatedFiles []string

	instDir := filepath.Join(dir, "inst")
	platforms, err := os.ReadDir(instDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	for _, platform := range platforms {
		if !platform.IsDir() {
			continue
		}

		nodesDir := filepath.Join(instDir, platform.Name(), "nodes")
		nodeFiles, err := os.ReadDir(nodesDir)
		if err != nil {
			continue
		}

		for _, nodeFile := range nodeFiles {
			if nodeFile.IsDir() || !strings.HasSuffix(nodeFile.Name(), ".yaml") {
				continue
			}

			nodePath := filepath.Join(nodesDir, nodeFile.Name())
			data, err := os.ReadFile(nodePath)
			if err != nil {
				continue
			}

			// Parse as yaml.Node to preserve formatting
			var doc yaml.Node
			if err := yaml.Unmarshal(data, &doc); err != nil {
				continue
			}

			updated := updateChassisInNode(&doc, oldChassis, newChassis)
			if updated {
				newData, err := yaml.Marshal(&doc)
				if err != nil {
					continue
				}
				if err := os.WriteFile(nodePath, newData, 0644); err != nil {
					continue
				}
				updatedFiles = append(updatedFiles, nodePath)
			}
		}
	}

	return updatedFiles, nil
}

// updateChassisInNode updates chassis array entries in a yaml.Node
func updateChassisInNode(node *yaml.Node, oldChassis, newChassis string) bool {
	updated := false

	switch node.Kind {
	case yaml.DocumentNode:
		for _, child := range node.Content {
			if updateChassisInNode(child, oldChassis, newChassis) {
				updated = true
			}
		}
	case yaml.MappingNode:
		for i := 0; i < len(node.Content); i += 2 {
			key := node.Content[i]
			value := node.Content[i+1]

			if key.Value == "chassis" && value.Kind == yaml.SequenceNode {
				// Update chassis array entries
				for _, item := range value.Content {
					if item.Kind == yaml.ScalarNode {
						if item.Value == oldChassis {
							item.Value = newChassis
							updated = true
						} else if strings.HasPrefix(item.Value, oldChassis+".") {
							item.Value = newChassis + item.Value[len(oldChassis):]
							updated = true
						}
					}
				}
			} else {
				if updateChassisInNode(value, oldChassis, newChassis) {
					updated = true
				}
			}
		}
	}

	return updated
}
