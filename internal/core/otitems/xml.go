package otitems

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

var ErrNoItems = errors.New("items.xml: no <items> element")

// ApplyXML reads names and types from an items.xml. Ids are server ids
// (client ids in ModeClient). Ids missing from the database are skipped.
func (db *DB) ApplyXML(data []byte) error {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	decoder.Strict = false
	decoder.CharsetReader = func(_ string, input io.Reader) (io.Reader, error) { return input, nil }
	type attrNode struct {
		Key   string `xml:"key,attr"`
		Value string `xml:"value,attr"`
	}
	type itemNode struct {
		ID           string     `xml:"id,attr"`
		FromID       string     `xml:"fromid,attr"`
		ToID         string     `xml:"toid,attr"`
		Name         string     `xml:"name,attr"`
		Article      string     `xml:"article,attr"`
		Plural       string     `xml:"plural,attr"`
		EditorSuffix string     `xml:"editorsuffix,attr"`
		Attrs        []attrNode `xml:"attribute"`
	}
	found := false
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("items.xml: %w", err)
		}
		start, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		switch strings.ToLower(start.Name.Local) {
		case "items":
			found = true
		case "item":
			var node itemNode
			if err := decoder.DecodeElement(&node, &start); err != nil {
				return fmt.Errorf("items.xml: %w", err)
			}
			from, to := parseRange(node.ID, node.FromID, node.ToID)
			for id := from; id <= to && id != 0; id++ {
				itemType := db.Get(uint16(id))
				if itemType == nil {
					if id == to {
						break
					}
					continue
				}
				if node.Name != "" {
					itemType.Name = node.Name
				}
				if node.Article != "" {
					itemType.Article = node.Article
				}
				if node.Plural != "" {
					itemType.Plural = node.Plural
				}
				if node.EditorSuffix != "" {
					itemType.EditorSuffix = node.EditorSuffix
				}
				for _, attr := range node.Attrs {
					applyXMLAttr(itemType, strings.ToLower(attr.Key), attr.Value)
				}
				if id == to {
					break
				}
			}
		}
	}
	if !found {
		return ErrNoItems
	}
	return nil
}

func parseRange(id, from, to string) (uint32, uint32) {
	parse := func(text string) uint32 {
		number, _ := strconv.ParseUint(strings.TrimSpace(text), 10, 16)
		return uint32(number)
	}
	if id != "" {
		single := parse(id)
		return single, single
	}
	first, last := parse(from), parse(to)
	if last < first {
		last = first
	}
	return first, last
}

func applyXMLAttr(itemType *Type, key, value string) {
	switch key {
	case "type":
		if kind, ok := kindNames[strings.ToLower(value)]; ok {
			itemType.Kind = kind
			if kind == KindContainer || kind == KindDepot || kind == KindMailbox || kind == KindTrashHolder {
				itemType.Flags |= FlagContainer
			}
		}
	case "rotateto":
		rotateTo, _ := strconv.ParseUint(value, 10, 16)
		itemType.RotateTo = uint16(rotateTo)
	case "containersize":
		size, _ := strconv.ParseUint(value, 10, 16)
		itemType.MaxItems = uint16(size)
		itemType.Flags |= FlagContainer
	case "charges":
		itemType.Flags |= FlagCharges
	case "corpsetype":
		itemType.Flags |= FlagCorpse
	case "writeable", "writable":
		itemType.Flags |= FlagWritable | FlagReadable
	case "readable":
		itemType.Flags |= FlagReadable
	}
}
