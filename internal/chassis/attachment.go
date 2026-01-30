package chassis

import (
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Attachment represents a component attached to a chassis section
type Attachment struct {
	Component string
	Playbook  string
	Section   string
}

// LoadAttachments scans playbooks for component attachments to a chassis section
func LoadAttachments(dir, section string) ([]Attachment, error) {
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
			// Match exact section or child sections
			if play.Hosts == section || strings.HasPrefix(play.Hosts, section+".") {
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
							Section:   play.Hosts,
						})
					}
				}
			}
		}
	}

	return attachments, nil
}

// HasAttachments checks if a chassis section has any component attachments
func HasAttachments(dir, section string) (bool, []Attachment, error) {
	attachments, err := LoadAttachments(dir, section)
	if err != nil {
		return false, nil, err
	}
	return len(attachments) > 0, attachments, nil
}

// UpdatePlaybookReferences renames chassis section references in all playbooks
func UpdatePlaybookReferences(dir, oldSection, newSection string) ([]string, error) {
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

		updated := updateHostsInNode(&doc, oldSection, newSection)
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
func updateHostsInNode(node *yaml.Node, oldSection, newSection string) bool {
	updated := false

	switch node.Kind {
	case yaml.DocumentNode:
		for _, child := range node.Content {
			if updateHostsInNode(child, oldSection, newSection) {
				updated = true
			}
		}
	case yaml.SequenceNode:
		for _, child := range node.Content {
			if updateHostsInNode(child, oldSection, newSection) {
				updated = true
			}
		}
	case yaml.MappingNode:
		for i := 0; i < len(node.Content); i += 2 {
			key := node.Content[i]
			value := node.Content[i+1]

			if key.Value == "hosts" && value.Kind == yaml.ScalarNode {
				// Check for exact match or prefix match
				if value.Value == oldSection {
					value.Value = newSection
					updated = true
				} else if strings.HasPrefix(value.Value, oldSection+".") {
					value.Value = newSection + value.Value[len(oldSection):]
					updated = true
				}
			} else {
				if updateHostsInNode(value, oldSection, newSection) {
					updated = true
				}
			}
		}
	}

	return updated
}
