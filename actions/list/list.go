package list

import (
	"sort"

	"github.com/launchrctl/launchr"
	"github.com/launchrctl/launchr/pkg/action"
	"github.com/plasmash/plasmactl-chassis/pkg/chassis"
	"github.com/plasmash/plasmactl-component/pkg/component"
	"github.com/plasmash/plasmactl-node/pkg/node"
)

// ListResult is the structured output for chassis:list
type ListResult struct {
	Chassis []string `json:"chassis"`
}

// List implements the chassis:list command
type List struct {
	action.WithLogger
	action.WithTerm

	Chassis string
	Tree    bool

	result *ListResult
}

// Result returns the structured result for JSON output
func (l *List) Result() any {
	return l.result
}

// Execute runs the list action
func (l *List) Execute() error {
	c, err := chassis.Load(".")
	if err != nil {
		return err
	}

	paths := c.FlattenWithPrefix(l.Chassis)
	if len(paths) == 0 {
		l.Term().Warning().Println("No chassis paths found")
		return nil
	}

	// Build result
	l.result = &ListResult{
		Chassis: paths,
	}

	if l.Tree {
		l.printTreeWithRelations(c, paths)
	} else {
		// Flat output - one per line, scriptable
		for _, c := range l.result.Chassis {
			l.Term().Printfln("%s", c)
		}
	}

	return nil
}


// printTreeWithRelations prints the chassis tree with nodes (🖥) and components (🧩) inline
func (l *List) printTreeWithRelations(c *chassis.Chassis, paths []string) {
	// Load nodes and compute allocations
	nodesByPlatform, _ := node.LoadByPlatform(".")
	chassisToNodes := make(map[string][]string)

	for _, nodes := range nodesByPlatform {
		allocations := nodes.Allocations(c)
		for _, n := range nodes {
			for _, chassisPath := range allocations[n.Hostname] {
				chassisToNodes[chassisPath] = append(chassisToNodes[chassisPath], n.DisplayName())
			}
		}
	}

	// Load components
	components, _ := component.LoadFromPlaybooks(".")
	chassisToComponents := make(map[string][]string)
	for _, comp := range components {
		chassisToComponents[comp.Chassis] = append(chassisToComponents[comp.Chassis], comp.Name)
	}

	// Sort the relations for consistent output
	for chassisPath := range chassisToNodes {
		sort.Strings(chassisToNodes[chassisPath])
	}
	for chassisPath := range chassisToComponents {
		sort.Strings(chassisToComponents[chassisPath])
	}

	// Build tree structure
	tree := buildTree(paths)

	// Print tree starting from root's children
	for _, child := range tree.children {
		printNodeWithRelations(l.Term(), child, "", "", chassisToNodes, chassisToComponents)
	}
}

type treeNode struct {
	name     string
	fullPath string
	children []*treeNode
}

func buildTree(paths []string) *treeNode {
	root := &treeNode{name: ""}

	for _, path := range paths {
		parts := splitPath(path)
		current := root
		currentPath := ""
		for _, part := range parts {
			if currentPath == "" {
				currentPath = part
			} else {
				currentPath = currentPath + "." + part
			}

			found := false
			for _, child := range current.children {
				if child.name == part {
					current = child
					found = true
					break
				}
			}
			if !found {
				newNode := &treeNode{name: part, fullPath: currentPath}
				current.children = append(current.children, newNode)
				current = newNode
			}
		}
	}

	return root
}

func splitPath(path string) []string {
	var parts []string
	current := ""
	for _, c := range path {
		if c == '.' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(c)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}

func printNodeWithRelations(term *launchr.Terminal, node *treeNode, indent, prefix string, chassisToNodes, chassisToComponents map[string][]string) {
	// Print this node
	term.Printfln("%s%s", prefix, node.name)

	// Get nodes and components for this chassis path
	nodes := chassisToNodes[node.fullPath]
	comps := chassisToComponents[node.fullPath]

	// Calculate total children (chassis + nodes + components)
	totalChildren := len(node.children) + len(nodes) + len(comps)
	childIdx := 0

	// Print nodes for this chassis path
	for _, n := range nodes {
		childIdx++
		isLast := childIdx == totalChildren
		var childPrefix, nextIndent string
		if isLast {
			childPrefix = indent + "└── "
			nextIndent = indent + "    "
		} else {
			childPrefix = indent + "├── "
			nextIndent = indent + "│   "
		}
		_ = nextIndent // unused for leaf nodes
		term.Printfln("%s🖥  %s", childPrefix, n)
	}

	// Print components for this chassis path
	for _, comp := range comps {
		childIdx++
		isLast := childIdx == totalChildren
		var childPrefix, nextIndent string
		if isLast {
			childPrefix = indent + "└── "
			nextIndent = indent + "    "
		} else {
			childPrefix = indent + "├── "
			nextIndent = indent + "│   "
		}
		_ = nextIndent // unused for leaf nodes
		term.Printfln("%s🧩 %s", childPrefix, comp)
	}

	// Print child chassis paths
	for _, child := range node.children {
		childIdx++
		isLast := childIdx == totalChildren

		var childPrefix, nextIndent string
		if isLast {
			childPrefix = indent + "└── "
			nextIndent = indent + "    "
		} else {
			childPrefix = indent + "├── "
			nextIndent = indent + "│   "
		}

		printNodeWithRelations(term, child, nextIndent, childPrefix, chassisToNodes, chassisToComponents)
	}
}
