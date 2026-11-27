package xmlwriter

import (
	"strconv"

	"github.com/polestorm/goxmlgo/xmlreader"
)

// Error codes of the writer (Error.Code). A code names a rule, not a package: where the writer refuses what the
// reader would reject, it uses the code of the reader. The codes below the blank line belong to the writer alone.
// Codes do not change, so a caller can translate messages by code; all of them are listed in docs/errors.md.
const (
	CodeEncodingInvalid     = xmlreader.CodeEncodingInvalid     // data that is not valid UTF-8
	CodeChar                = xmlreader.CodeChar                // a character outside [2] Char
	CodeName                = xmlreader.CodeName                // a name that does not match [5] Name
	CodeQName               = xmlreader.CodeQName               // a colon where a name without one is required
	CodeReservedPITarget    = xmlreader.CodeReservedPITarget    // a PI target matching [Xx][Mm][Ll]
	CodeXMLDecl             = xmlreader.CodeXMLDecl             // Declaration with a bad value, or not first
	CodeNoRoot              = xmlreader.CodeNoRoot              // Close before the root element
	CodeEOF                 = xmlreader.CodeEOF                 // Close with elements still open
	CodeOutsideRoot         = xmlreader.CodeOutsideRoot         // a second root element, or text outside the root
	CodeCDATAEnd            = xmlreader.CodeCDATAEnd            // "]]>" in the data of a CDATA section
	CodeComment             = xmlreader.CodeComment             // "--" in a comment, or "-" at its end
	CodeElementTypeMatch    = xmlreader.CodeElementTypeMatch    // an end token whose name is not that of the open element
	CodeUniqueAttSpec       = xmlreader.CodeUniqueAttSpec       // an attribute or a namespace prefix twice in one tag
	CodeAttributesUnique    = xmlreader.CodeAttributesUnique    // two attributes with the same expanded name
	CodePrefixDeclared      = xmlreader.CodePrefixDeclared      // a prefix that is not declared in scope
	CodeNoPrefixUndeclaring = xmlreader.CodeNoPrefixUndeclaring // a declaration of a prefix with an empty namespace name
	CodeReservedPrefixes    = xmlreader.CodeReservedPrefixes    // misuse of the prefixes xml and xmlns or their namespaces
	CodeLimitDepth          = xmlreader.CodeLimitDepth
	CodeLimitNameLength     = xmlreader.CodeLimitNameLength
	CodeLimitAttributes     = xmlreader.CodeLimitAttributes
	CodeLimitTextSize       = xmlreader.CodeLimitTextSize

	CodePIData        = "xml-pi-data"                 // "?>" in the data of a PI, or white space at its start
	CodeNamespaceName = "ns-name-mismatch"            // Name.Space is not what Name.Prefix is bound to
	CodeLineBreak     = "unsupported-carriage-return" // a carriage return where no reader would return it
	CodeDoctype       = "unsupported-doctype"         // a DOCTYPE token with Options.RejectDoctype
	CodeUsageClosed   = "usage-closed"                // a call after Close
	CodeUsageAttr     = "usage-attr"                  // Attr when no start tag is open
	CodeUsageEnd      = "usage-end"                   // End without an open element, EndEmpty after content
	CodeUsageToken    = "usage-token"                 // a token of an unknown kind
	CodeUsageOptions  = "usage-options"               // Options that contradict each other or the call
)

// Error is an error of the writer about a call: the call asked for something that would not be a well-formed
// document, or that a reader would not give back unchanged. Nothing of that call was written and the Writer is
// as it was before the call, so the caller may go on. Errors of the io.Writer are returned as they are, not as
// *Error.
type Error struct {
	Code string // one of the Code* constants
	// Path is the path of open elements at the time of the call, as qualified names: "/" at the level of the
	// document, "/a/p:b" inside <p:b> in <a>.
	Path string
	// Call is the number of the call on this Writer that failed, from 1, counted from New or Reset.
	Call    int
	Message string
}

func (e *Error) Error() string {
	return "xmlwriter: " + e.Path + ": call " + strconv.Itoa(e.Call) + ": " + e.Code + ": " + e.Message
}
