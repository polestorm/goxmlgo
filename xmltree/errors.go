package xmltree

import (
	"strconv"

	"github.com/polestorm/goxmlgo/xmlreader"
)

// Error codes of this package. An error of the source of tokens (*xmlreader.Error) and an error of the writer
// (*xmlwriter.Error) are returned as they are, with their own codes.
const (
	// Codes of the reader that the tree uses, with the meaning of the reader.
	CodeOutsideRoot = xmlreader.CodeOutsideRoot
	CodeNoRoot      = xmlreader.CodeNoRoot

	CodeLimitNodes  = "limit-nodes"
	CodeIDDuplicate = "id-duplicate"
	CodeHasParent   = "tree-has-parent"
	CodeCycle       = "tree-cycle"
	CodeNodeKind    = "tree-node-kind"
	CodeNotChild    = "tree-not-child"
	CodeSource      = "tree-source"
)

// Error is an error of the tree: a limit, a repeated identifier, a change that the tree refuses, or a source of
// tokens that did not give the tokens of a document.
type Error struct {
	// Code is one of the Code constants; it never changes for a given kind of error.
	Code string
	// Line and Column are the place in the source of the node or attribute the error is about, from 1; zero
	// when the node was not read from a source.
	Line, Column int
	// FirstLine and FirstColumn (CodeIDDuplicate): the place of the first attribute with the same value.
	FirstLine, FirstColumn int
	// Message describes the error in English; it may change between versions.
	Message string
}

func (e *Error) Error() string {
	if e.Line == 0 {
		return e.Code + ": " + e.Message
	}
	return strconv.Itoa(e.Line) + ":" + strconv.Itoa(e.Column) + ": " + e.Code + ": " + e.Message
}

func nodeErr(code string, n *Node, msg string) error {
	e := &Error{Code: code, Message: msg}
	if n != nil {
		e.Line, e.Column = n.line, n.column
	}
	return e
}
