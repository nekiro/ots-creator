// Package xmltree parses XML into a plain element tree. It is lenient like
// the servers and editors that write the data files: names are lower case,
// any charset is read as is, NUL bytes are dropped and whatever follows the
// root element is ignored.
package xmltree

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"math"
	"strconv"
	"strings"
)

// ErrEmpty is returned for a document without elements.
var ErrEmpty = errors.New("empty document")

// Node is one element. Element and attribute names are lower case.
type Node struct {
	Name     string
	Attrs    map[string]string
	Children []*Node
}

// Parse reads the root element of data.
func Parse(data []byte) (*Node, error) {
	// Some data files end with a NUL byte, which RME accepts.
	data = bytes.ReplaceAll(data, []byte{0}, nil)
	decoder := xml.NewDecoder(bytes.NewReader(data))
	decoder.Strict = false
	decoder.CharsetReader = func(_ string, input io.Reader) (io.Reader, error) { return input, nil }
	var open []*Node
	var root *Node
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch element := token.(type) {
		case xml.StartElement:
			node := &Node{Name: strings.ToLower(element.Name.Local), Attrs: map[string]string{}}
			for _, attr := range element.Attr {
				node.Attrs[strings.ToLower(attr.Name.Local)] = attr.Value
			}
			if len(open) > 0 {
				parent := open[len(open)-1]
				parent.Children = append(parent.Children, node)
			} else {
				root = node
			}
			open = append(open, node)
		case xml.EndElement:
			if len(open) > 0 {
				open = open[:len(open)-1]
			}
			if len(open) == 0 && root != nil {
				return root, nil
			}
		}
	}
	if root == nil {
		return nil, ErrEmpty
	}
	return root, nil
}

// Str returns an attribute, or "" when it is missing.
func (n *Node) Str(key string) string { return n.Attrs[key] }

// Has reports whether the attribute is present.
func (n *Node) Has(key string) bool {
	_, ok := n.Attrs[key]
	return ok
}

// Int returns a numeric attribute; a fraction is truncated and anything
// else that is not a number is 0.
func (n *Node) Int(key string) int {
	text := strings.TrimSpace(n.Attrs[key])
	if value, err := strconv.Atoi(text); err == nil {
		return value
	}
	if value, err := strconv.ParseFloat(text, 64); err == nil {
		return int(value)
	}
	return 0
}

// U16 returns a numeric attribute in uint16 range, otherwise 0.
func (n *Node) U16(key string) uint16 {
	value := n.Int(key)
	if value < 0 || value > math.MaxUint16 {
		return 0
	}
	return uint16(value)
}

// Bool reports whether an attribute is "true", "yes" or "1".
func (n *Node) Bool(key string) bool {
	switch strings.ToLower(strings.TrimSpace(n.Attrs[key])) {
	case "true", "yes", "1":
		return true
	}
	return false
}
