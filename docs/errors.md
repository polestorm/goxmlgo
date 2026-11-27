# Error codes

Every error of the library has a fixed code. Messages (`Error.Message`) are in English and may change between
versions; the contract is the code and the position, so a caller can translate or classify errors by code. A code,
once released, keeps its meaning and is not removed. `TestErrorCatalog` checks that this file lists exactly the
codes the source defines.

The catalogue grows with the packages: `xmlreader` (v0.1.0), `xmlwriter` (v0.3.0), `xmltree` (v0.4.0).

A code names a rule, not a package. Where the writer refuses what the reader would reject, it reports the code of
the reader, so each code is listed once, under the package that introduced it; the section on `xmlwriter` says
which codes of the reader the writer uses.

## xmlreader

`Reader.Next` and `Reader.Skip` return `*xmlreader.Error` with `Code`, `Line`, `Column` (both from 1, the column in
characters) and `Message`. Every error about the document is fatal (XML 1.0, section 1.2): after it `Next` keeps
returning the same error. An
error inside the replacement text of an entity points at the reference to the entity in the document.

Sources: [XML] is Extensible Markup Language (XML) 1.0 Fifth Edition, [NS] is Namespaces in XML 1.0 Third Edition.
Numbers in square brackets are grammar productions of [XML]. The codes of v0.1.0 are those of goxsdgo/xmlreader
v0.3.0; `limit-elements` and `usage-skip` were added in v0.2.0.

### Grammar and document structure (`xml-*`)

| Code | Constant | Rule | Meaning |
|---|---|---|---|
| `xml-encoding-invalid` | `CodeEncodingInvalid` | [XML] 4.3.3 | Bytes that are not valid in the encoding of the document, or UTF-16 without a byte order mark. Reported at the first bad byte. |
| `xml-encoding-unsupported` | `CodeEncodingUnsupported` | [XML] 4.3.3 | An encoding other than UTF-8, UTF-16, ISO-8859-1 or US-ASCII (UTF-32 and EBCDIC included). |
| `xml-encoding-mismatch` | `CodeEncodingMismatch` | [XML] 4.3.3 | The encoding declaration contradicts the encoding detected from the byte order mark. |
| `xml-char` | `CodeChar` | [XML] [2] Char | A character that is not allowed in an XML document. |
| `xml-syntax` | `CodeSyntax` | [XML] grammar | A violation of a grammar production that has no code of its own. |
| `xml-eof` | `CodeEOF` | [XML] grammar | The document ends inside a construct. |
| `xml-name` | `CodeName` | [XML] [5] Name | A name that does not match Name (element, attribute, entity, notation, PI target). |
| `xml-no-root` | `CodeNoRoot` | [XML] [1] document | The document has no root element. |
| `xml-content-outside-root` | `CodeOutsideRoot` | [XML] [1], [27] Misc | Content other than comments, processing instructions and white space outside the root element. |
| `xml-cdata-end` | `CodeCDATAEnd` | [XML] [14] CharData | `]]>` in character data. |
| `xml-comment` | `CodeComment` | [XML] [15] Comment | `--` inside a comment. |
| `xml-reserved-pi-target` | `CodeReservedPITarget` | [XML] [17] PITarget | A processing instruction target that matches `xml` in any letter case. |
| `xml-decl` | `CodeXMLDecl` | [XML] [23]–[26], [32], [80], [81] | A malformed XML declaration, or one that is not at the very start of the document. |
| `xml-entity-structure` | `CodeEntityStructure` | [XML] 4.3.2 | The replacement text of an entity used in content is not well-formed content (for example an element left open). |

### Well-formedness constraints (`wfc-*`)

The code is the name of the constraint in [XML].

| Code | Constant | Rule | Meaning |
|---|---|---|---|
| `wfc-element-type-match` | `CodeElementTypeMatch` | WFC: Element Type Match | The name in an end tag differs from the name in the start tag. |
| `wfc-unique-att-spec` | `CodeUniqueAttSpec` | WFC: Unique Att Spec | An attribute name appears twice in one tag. |
| `wfc-no-external-entity-references` | `CodeNoExternalEntityRef` | WFC: No External Entity References | An attribute value refers to an external entity. |
| `wfc-no-lt-in-attribute-values` | `CodeNoLtInAttValue` | WFC: No < in Attribute Values | The replacement text of an entity referred to in an attribute value contains `<`. |
| `wfc-legal-character` | `CodeLegalCharacter` | WFC: Legal Character | A character reference to a character outside [2] Char. |
| `wfc-entity-declared` | `CodeEntityDeclared` | WFC: Entity Declared | A reference to an entity that is not declared, where the constraint applies (no external subset and no parameter entity references, or `standalone="yes"`). |
| `wfc-parsed-entity` | `CodeParsedEntity` | WFC: Parsed Entity | A reference to an unparsed entity. |
| `wfc-no-recursion` | `CodeNoRecursion` | WFC: No Recursion | An entity refers to itself, directly or indirectly. |
| `wfc-pes-in-internal-subset` | `CodePEsInInternalSubset` | WFC: PEs in Internal Subset | A parameter entity reference inside a markup declaration in the internal subset. |

### Namespaces (`nsc-*`, `ns-*`)

`nsc-*` codes are the names of the namespace constraints in [NS].

| Code | Constant | Rule | Meaning |
|---|---|---|---|
| `nsc-prefix-declared` | `CodePrefixDeclared` | NSC: Prefix Declared | A prefix that is not declared in scope. |
| `nsc-no-prefix-undeclaring` | `CodeNoPrefixUndeclaring` | NSC: No Prefix Undeclaring | `xmlns:p=""`. |
| `nsc-attributes-unique` | `CodeAttributesUnique` | NSC: Attributes Unique | Two attributes of an element have the same expanded name. |
| `nsc-reserved-prefixes` | `CodeReservedPrefixes` | NSC: Reserved Prefixes and Namespace Names | A declaration that misuses the prefixes `xml` or `xmlns` or their namespace names. |
| `ns-qname` | `CodeQName` | [NS] 4, 7 | An element or attribute name that is not a QName, or a colon in the name of an entity, PI target or notation. |

### Limits (`limit-*`)

Limits are on by default (`xmlreader.DefaultLimits`); zero in `Limits` means the default, a negative value no
limit.

| Code | Constant | Limit | Default |
|---|---|---|---|
| `limit-depth` | `CodeLimitDepth` | `MaxDepth`: nesting of elements | 512 |
| `limit-name-length` | `CodeLimitNameLength` | `MaxNameLength`: length of a name in bytes | 16 KiB |
| `limit-attributes` | `CodeLimitAttributes` | `MaxAttributes`: attributes of one element, namespace declarations included | 4096 |
| `limit-text-size` | `CodeLimitTextSize` | `MaxTextSize`: one text, comment or PI token, or one attribute value, in bytes | 16 MiB |
| `limit-entity-expansion` | `CodeLimitEntityExpand` | `MaxEntityExpansion`: total length of the replacement text of expanded entities | 4 MiB |
| `limit-entity-depth` | `CodeLimitEntityDepth` | `MaxEntityDepth`: nesting of entity references | 32 |
| `limit-dtd-size` | `CodeLimitDTDSize` | `MaxDTDSize`: length of the DOCTYPE declaration with its internal subset | 1 MiB |
| `limit-elements` | `CodeLimitElements` | `MaxElements`: elements in the whole document, those passed over by `Skip` included; reported at the start tag that exceeds it | no limit (zero and negative values) |
| `limit-namespaces` | `CodeLimitNamespaces` | `MaxNamespaces`: namespace declarations in scope at one time, those of all open elements together, default 256 (added in v0.4.0). The position is that of the element whose declarations go over the limit. |

### Well-formed constructs the reader does not process (`unsupported-*`)

The reader never reads external entities or the external subset. Where skipping content silently would change
what the caller sees, it reports an error instead ([XML] 5.1 lets a non-validating processor do either).

| Code | Constant | Rule | Meaning |
|---|---|---|---|
| `unsupported-external-entity` | `CodeExternalEntity` | [XML] 4.4.3, 5.1 | A reference to an external general entity in content. |
| `unsupported-entity-unresolved` | `CodeEntityUnresolved` | [XML] 5.1 | A reference to an entity that is not declared in the internal subset and may be declared in the external subset or in a parameter entity, which are not read. |
| `unsupported-conditional-section` | `CodeConditionalSection` | [XML] 3.4 | A conditional section in the replacement text of a parameter entity. |

### Calls the state of the reader does not allow (`usage-*`)

These say nothing about the document and are not fatal: the reader stays as it was.

| Code | Constant | Meaning |
|---|---|---|
| `usage-skip` | `CodeUsageSkip` | `Skip` was called when the last token returned by `Next` was not a start tag (or before the first token). |

## xmlwriter

The methods of `xmlwriter.Writer` return `*xmlwriter.Error` with `Code`, `Path` (the open elements at the time of
the call, `/a/p:b`; `/` at the level of the document), `Call` (the number of the failed call on this Writer, from
1) and `Message`. An error of the writer is never fatal: nothing of the failed call was written, the Writer is as
it was before the call, and the caller may go on. An error of the `io.Writer` is not an `*Error`: it is returned
as it is, by that call and by every call after it.

Sources as above. The codes were added in v0.3.0.

### Codes of the reader that the writer uses

The meaning is that of the reader, applied to what the caller asks the writer to write.

| Constant (`xmlwriter.`) | Code | When the writer reports it |
|---|---|---|
| `CodeEncodingInvalid` | `xml-encoding-invalid` | Data or a namespace name that is not valid UTF-8. |
| `CodeChar` | `xml-char` | A character outside [2] Char in text, an attribute value, a namespace name, a comment, a PI or a CDATA section. |
| `CodeName` | `xml-name` | An element name, attribute name, namespace prefix or PI target that does not match [5] Name. |
| `CodeQName` | `ns-qname` | A colon in a local name, a prefix or a PI target, with namespace processing. |
| `CodeReservedPITarget` | `xml-reserved-pi-target` | A PI target that matches `xml` in any letter case. |
| `CodeXMLDecl` | `xml-decl` | `Declaration` after something was written, or with a `standalone` other than `yes`, `no` or empty. |
| `CodeNoRoot` | `xml-no-root` | `Close` before the root element. |
| `CodeEOF` | `xml-eof` | `Close` with elements still open. |
| `CodeOutsideRoot` | `xml-content-outside-root` | A second root element; text other than white space, or a CDATA section, outside the root element. |
| `CodeCDATAEnd` | `xml-cdata-end` | `]]>` in the data of a CDATA section (the writer does not split sections). |
| `CodeComment` | `xml-comment` | `--` in a comment, or `-` at its end. |
| `CodeElementTypeMatch` | `wfc-element-type-match` | An end token whose name is not that of the open element. |
| `CodeUniqueAttSpec` | `wfc-unique-att-spec` | An attribute given twice in one tag, or a namespace prefix declared twice in one tag. |
| `CodeAttributesUnique` | `nsc-attributes-unique` | Two attributes of one tag with the same expanded name and different prefixes. |
| `CodePrefixDeclared` | `nsc-prefix-declared` | A name with a prefix that is not declared in scope (without `AutoNamespaces`). |
| `CodeNoPrefixUndeclaring` | `nsc-no-prefix-undeclaring` | A declaration of a prefix with an empty namespace name. |
| `CodeReservedPrefixes` | `nsc-reserved-prefixes` | A declaration or name that misuses the prefixes `xml` or `xmlns` or their namespace names; a namespace declaration passed as an attribute. |
| `CodeLimitDepth` | `limit-depth` | `Limits.MaxDepth`, default 512. |
| `CodeLimitNameLength` | `limit-name-length` | `Limits.MaxNameLength`: a qualified name or a PI target, default 16 KiB. |
| `CodeLimitAttributes` | `limit-attributes` | `Limits.MaxAttributes`: attributes of one element, namespace declarations included, default 4096. |
| `CodeLimitTextSize` | `limit-text-size` | `Limits.MaxTextSize`: a run of text between two tags (several `Text` calls add up), a CDATA section, a comment, the data of a PI, an attribute value or a namespace name, default 16 MiB. |
| `CodeLimitNamespaces` | `limit-namespaces` | `Limits.MaxNamespaces`: namespace declarations in scope at one time, those the writer adds with `AutoNamespaces` included, default 256. |

The limits of the writer have the defaults of the reader, so a document written with default limits can be read
with default limits. Zero in `xmlwriter.Limits` means the default, a negative value no limit.

### Codes of the writer

| Code | Constant | Rule | Meaning |
|---|---|---|---|
| `xml-pi-data` | `CodePIData` | [XML] [16] PI | `?>` in the data of a processing instruction, or white space at its start (a reader returns the data without the white space after the target, so it would not come back). |
| `ns-name-mismatch` | `CodeNamespaceName` | [NS] 3, 6 | `Name.Space` is not the namespace `Name.Prefix` is bound to in scope: a prefix bound to another namespace, an element in a namespace that is not the default one, an element without a namespace under a default namespace, an attribute in a namespace without a prefix, a prefix on a name without a namespace. With `AutoNamespaces` only what a declaration cannot fix. |
| `unsupported-carriage-return` | `CodeLineBreak` | [XML] 2.11 | A carriage return in a comment, a PI, a CDATA section or in white space outside the root element. No character reference is possible there, and a reader turns the character into a line feed. In text and attribute values a carriage return is written as `&#13;`. |
| `unsupported-doctype` | `CodeDoctype` | [XML] 2.8 | `WriteToken` got a DOCTYPE token and `Options.RejectDoctype` is set. By default the token is skipped: the writer does not write DOCTYPE declarations. |

### Calls the state of the writer does not allow (`usage-*`)

| Code | Constant | Meaning |
|---|---|---|
| `usage-closed` | `CodeUsageClosed` | A call after a successful `Close`, until `Reset`. |
| `usage-attr` | `CodeUsageAttr` | `Attr` when no start tag is open: before `Start`, or after content or `End`. |
| `usage-end` | `CodeUsageEnd` | `End` without an open element; `EndEmpty` after content. |
| `usage-token` | `CodeUsageToken` | `WriteToken` got a token of an unknown kind. |
| `usage-options` | `CodeUsageOptions` | `Options` that contradict each other or the call: `Indent` with characters other than spaces and tabs, `AutoNamespaces` with `NoNamespaces`; with `NoNamespaces`, a name with a namespace or prefix, or namespace declarations. |

## xmltree

The functions and methods of `xmltree` return `*xmltree.Error` with `Code`, `Line` and `Column` (the place in the
source of the node or attribute the error is about; zero for a node that was not read from a source),
`FirstLine` and `FirstColumn` (for `id-duplicate`) and `Message`. An error of the source of tokens
(`*xmlreader.Error`) and an error of the writer (`*xmlwriter.Error`) are returned as they are, with their own
codes. A change that the tree refuses changes nothing.

The codes were added in v0.4.0. `limit-nodes` and `id-duplicate` have the names they had in the tree of goxadesgo.

### Codes of the reader that the tree uses

| Constant (`xmltree.`) | Code | When the tree reports it |
|---|---|---|
| `CodeOutsideRoot` | `xml-content-outside-root` | A second root element, or text other than white space, put at the top level of a document (`Append`, `InsertBefore`, `Replace`) or given there by a source of tokens. |
| `CodeNoRoot` | `xml-no-root` | `ReadDocument` got no root element from the source. |

### Codes of the tree

| Code | Constant | Meaning |
|---|---|---|
| `limit-nodes` | `CodeLimitNodes` | The source has more nodes (elements, text nodes, comments, processing instructions) than `Options.MaxNodes`, default 2 000 000. The position is that of the first node over the limit; no tree is returned. |
| `id-duplicate` | `CodeIDDuplicate` | Two elements have the same identifier (`Options.IDs`, `Options.IDAttributes`, `Document.IndexIDs`). `Line` and `Column` are the place of the second attribute, `FirstLine` and `FirstColumn` of the first. |
| `tree-has-parent` | `CodeHasParent` | A node that has a parent was given as a new child. Remove it first, or clone it. |
| `tree-cycle` | `CodeCycle` | A node was put into itself or into one of its descendants. |
| `tree-node-kind` | `CodeNodeKind` | The operation does not fit the kind of node: a child for a node that is not an element or a document, a document node or nil as a child, a method that the kind of node does not have (`SetText`, `SetAttr`, `SetAttrs`, `SetNamespaces`, `DeclareNamespace`, `SetSelfClosing` on something other than an element; `SetName` on a node without a name; `SetData` on an element or a document; `SetCDATA` on something other than text), the zero `Node`. |
| `tree-not-child` | `CodeNotChild` | The node given to `Remove`, `Replace` or as the reference of `InsertBefore` is not a child of the node. |
| `tree-source` | `CodeSource` | The source of tokens did not give the tokens of a document: it ended inside an element, gave an end tag without a start tag or a token of an unknown kind, or `ReadElement` was called with a token that is not a start tag. |
