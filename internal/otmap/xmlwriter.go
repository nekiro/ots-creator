package otmap

import (
	"bytes"
	"encoding/xml"
	"strconv"
	"strings"
)

// xmlWriter writes simple indented XML.
type xmlWriter struct {
	buf   bytes.Buffer
	depth int
}

func newXMLWriter() *xmlWriter {
	w := &xmlWriter{}
	w.buf.WriteString("<?xml version=\"1.0\"?>\n")
	return w
}

// xmlAttr is one attribute of an element.
type xmlAttr struct {
	name  string
	value string
}

func (w *xmlWriter) elem(name string, attrs []xmlAttr, selfClose bool) {
	w.buf.WriteString(strings.Repeat("\t", w.depth))
	w.buf.WriteString("<" + name)
	for _, attr := range attrs {
		w.buf.WriteString(" " + attr.name + "=\"")
		xml.EscapeText(&w.buf, []byte(attr.value))
		w.buf.WriteString("\"")
	}
	if selfClose {
		w.buf.WriteString(" />\n")
		return
	}
	w.buf.WriteString(">\n")
	w.depth++
}

func (w *xmlWriter) end(name string) {
	w.depth--
	w.buf.WriteString(strings.Repeat("\t", w.depth) + "</" + name + ">\n")
}

func itoa[T ~int | ~int32 | ~uint32 | ~uint16 | ~uint8](value T) string {
	return strconv.Itoa(int(value))
}
