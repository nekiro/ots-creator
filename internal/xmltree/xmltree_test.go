package xmltree

import (
	"errors"
	"testing"
)

func TestParse(t *testing.T) {
	root, err := Parse([]byte("<?xml version=\"1.0\" encoding=\"ISO-8859-1\"?>\n" +
		"<Houses><House Name=\"Main\" ID=\"7\"><Door x=\"1\"/></House><house/></Houses>\n<trailing>\x00"))
	if err != nil {
		t.Fatal(err)
	}
	if root.Name != "houses" || len(root.Children) != 2 {
		t.Fatalf("root = %+v", root)
	}
	house := root.Children[0]
	if house.Name != "house" || house.Str("name") != "Main" || house.Int("id") != 7 || len(house.Children) != 1 {
		t.Fatalf("house = %+v", house)
	}
	if _, err := Parse([]byte("  ")); !errors.Is(err, ErrEmpty) {
		t.Fatalf("empty err = %v", err)
	}
	if _, err := Parse([]byte("<a><b")); err == nil {
		t.Fatal("truncated document parsed")
	}
}

func TestAttributes(t *testing.T) {
	node := &Node{Attrs: map[string]string{
		"int": " 12 ", "float": "2.7", "text": "abc", "big": "70000", "negative": "-1",
		"yes": "Yes", "one": "1", "no": "false",
	}}
	if node.Int("int") != 12 || node.Int("float") != 2 || node.Int("text") != 0 || node.Int("missing") != 0 {
		t.Fatal("Int")
	}
	if node.U16("int") != 12 || node.U16("big") != 0 || node.U16("negative") != 0 {
		t.Fatal("U16")
	}
	if !node.Bool("yes") || !node.Bool("one") || node.Bool("no") || node.Bool("missing") {
		t.Fatal("Bool")
	}
	if !node.Has("text") || node.Has("missing") || node.Str("text") != "abc" {
		t.Fatal("Has / Str")
	}
}
