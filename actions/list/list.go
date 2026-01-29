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
	for _, child := range tree.children {
		printNode(child, "")
	}
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

func printNode(node *treeNode, indent string) {
	// Print this node
	fmt.Println(node.name)

	// Print children with tree structure
	for i, child := range node.children {
		isLast := i == len(node.children)-1

		var prefix, nextIndent string
		if isLast {
			prefix = indent + "└── "
			nextIndent = indent + "    "
		} else {
			prefix = indent + "├── "
			nextIndent = indent + "│   "
		}

		fmt.Print(prefix)
		printNodeWithIndent(child, nextIndent)
	}
}

func printNodeWithIndent(node *treeNode, indent string) {
	// Print this node's name (prefix already printed)
	fmt.Println(node.name)

	// Print children
	for i, child := range node.children {
		isLast := i == len(node.children)-1

		var prefix, nextIndent string
		if isLast {
			prefix = indent + "└── "
			nextIndent = indent + "    "
		} else {
			prefix = indent + "├── "
			nextIndent = indent + "│   "
		}

		fmt.Print(prefix)
		printNodeWithIndent(child, nextIndent)
	}
}
