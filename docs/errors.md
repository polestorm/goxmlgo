# Error codes

Every error of the library has a fixed code. Messages (`Error.Message`) are in English and may change between
versions; the contract is the code and the position, so a caller can translate or classify errors by code. A code,
once released, keeps its meaning and is not removed. `TestErrorCatalog` checks that this file lists exactly the
codes the source defines.

The catalogue grows with the packages: `xmlreader` (v0.1.0, below), `xmlwriter` (stage 3), `xmltree` (stage 4).

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
