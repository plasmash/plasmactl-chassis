package list

import (
	"fmt"

	"github.com/launchrctl/launchr/pkg/action"
	"github.com/plasmash/plasmactl-chassis/internal/chassis"
)

// List implements the chassis:list command
type List struct {
	action.WithLogger
	action.WithTerm

	Section string
	Tree    bool
}

// Execute runs the list action
func (l *List) Execute() error {
	c, err := chassis.Load(".")
	if err != nil {
		return err
	}

	sections := c.FlattenWithPrefix(l.Section)
	if len(sections) == 0 {
		l.Term().Warning().Println("No chassis sections found")
		return nil
	}

	if l.Tree {
		l.printTree(sections)
	} else {
		l.printFlat(sections)
	}

	return nil
}

func (l *List) printFlat(sections []string) {
	for _, section := range sections {
		fmt.Println(section)
	}
}

func (l *List) printTree(sections []string) {
	// Build tree structure from paths
	tree := buildTree(sections)
	// Print tree starting from root's children
	printChildren(tree.children, "")
}

type treeNode struct {
	name     string
	children []*treeNode
}

func buildTree(paths []string) *treeNode {
	root := &treeNode{name: ""}

	for _, path := range paths {
		parts := splitPath(path)
		current := root
		for _, part := range parts {
			found := false
			for _, child := range current.children {
				if child.name == part {
					current = child
					found = true
					break
				}
			}
			if !found {
				newNode := &treeNode{name: part}
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

func printChildren(children []*treeNode, indent string) {
	for i, child := range children {
		isLast := i == len(children)-1

		// Determine the prefix and indent for this node
		var prefix, childIndent string
		if indent == "" {
			// Top level - no tree connectors
			prefix = ""
			childIndent = ""
		} else if isLast {
			prefix = indent + "└── "
			childIndent = indent + "    "
		} else {
			prefix = indent + "├── "
			childIndent = indent + "│   "
		}

		// Print this node
		fmt.Println(prefix + child.name)

		// Print children with updated indent
		if len(child.children) > 0 {
			if indent == "" {
				// First level children get tree connectors
				printChildren(child.children, "")
			} else {
				printChildren(child.children, childIndent)
			}
		}
	}
}
