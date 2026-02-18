package list

import (
	"sort"
	"strings"

	"github.com/launchrctl/launchr"
	"github.com/launchrctl/launchr/pkg/action"
	"github.com/plasmash/plasmactl-chassis/pkg/chassis"
	"github.com/plasmash/plasmactl-component/pkg/component"
	"github.com/plasmash/plasmactl-node/pkg/node"
)

// TreeEntry enriches a chassis path with its allocated nodes and attached components.
type TreeEntry struct {
	Path       string   `json:"path"`
	Nodes      []string `json:"nodes,omitempty"`
	Components []string `json:"components,omitempty"`
}

// ListResult is the structured output for chassis:list
type ListResult struct {
	Chassis []string    `json:"chassis"`
	Tree    []TreeEntry `json:"tree,omitempty"`
}

// List implements the chassis:list command
type List struct {
	action.WithLogger
	action.WithTerm

	Dir     string
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
	c, err := chassis.Load(l.Dir)
	if err != nil {
		return err
	}

	// Initialize result early so --json always returns an object, never null
	l.result = &ListResult{Chassis: []string{}}

	paths := c.FlattenWithPrefix(l.Chassis)
	if len(paths) == 0 {
		l.Term().Warning().Println("No chassis paths found")
		return nil
	}

	l.result.Chassis = paths

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
	nodesByPlatform, err := node.LoadByPlatform(l.Dir)
	if err != nil {
		l.Log().Debug("Failed to load nodes", "error", err)
	}
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
	components, err := component.LoadFromPlaybooks(l.Dir)
	if err != nil {
		l.Log().Debug("Failed to load components", "error", err)
	}
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

	// Populate tree entries in result
	for _, p := range paths {
		entry := TreeEntry{Path: p}
		if nodes, ok := chassisToNodes[p]; ok {
			entry.Nodes = nodes
		}
		if comps, ok := chassisToComponents[p]; ok {
			entry.Components = comps
		}
		l.result.Tree = append(l.result.Tree, entry)
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
		parts := strings.Split(path, ".")
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

func printNodeWithRelations(term *launchr.Terminal, node *treeNode, indent, prefix string, chassisToNodes, chassisToComponents map[string][]string) {
	// Print this node
	term.Printfln("%s%s", prefix, node.name)

	// Get nodes and components for this chassis path
	nodes := chassisToNodes[node.fullPath]
	comps := chassisToComponents[node.fullPath]

	// Order: child chassis paths first (structural hierarchy), then nodes, then components
	totalChildren := len(node.children) + len(nodes) + len(comps)
	childIdx := 0

	// Print child chassis paths first
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

	// Print nodes allocated to this chassis path
	for _, n := range nodes {
		childIdx++
		isLast := childIdx == totalChildren
		var childPrefix string
		if isLast {
			childPrefix = indent + "└── "
		} else {
			childPrefix = indent + "├── "
		}
		term.Printfln("%s🖥 %s", childPrefix, n)
	}

	// Print components distributed to this chassis path
	for _, comp := range comps {
		childIdx++
		isLast := childIdx == totalChildren
		var childPrefix string
		if isLast {
			childPrefix = indent + "└── "
		} else {
			childPrefix = indent + "├── "
		}
		term.Printfln("%s🧩 %s", childPrefix, comp)
	}
}
