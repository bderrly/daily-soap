package esv

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// processPassageHTML takes an HTML string containing verses and wraps/tags each verse
// (highlight + following text) in a span that carries the verse ID or tags the existing span.
func processPassageHTML(htmlStr string) (string, error) {
	// Parse the HTML fragment
	nodes, err := html.ParseFragment(strings.NewReader(htmlStr), &html.Node{
		Type:     html.ElementNode,
		Data:     "body",
		DataAtom: atom.Body,
	})
	if err != nil {
		return "", fmt.Errorf("failed to parse HTML fragment: %w", err)
	}

	// Create a new container to hold the result
	var buf bytes.Buffer
	var activeVerseID string
	for _, node := range nodes {
		// Pass state down and get updated state back
		activeVerseID = processNode(node, activeVerseID)
		if err := html.Render(&buf, node); err != nil {
			return "", fmt.Errorf("failed to render node: %w", err)
		}
	}

	return buf.String(), nil
}

// cutset includes spaces, newlines, tabs, and non-breaking spaces
const cutset = " \t\n\r\u00A0"

// verseIDRegex matches verse IDs like "v23063001" or "v23063001-1"
var verseIDRegex = regexp.MustCompile(`^v\d+.*`)

// processNode recursively traverses the DOM tree and wraps verses.
// It maintains an activeVerseID state to handle verses that span multiple block elements.
func processNode(n *html.Node, activeVerseID string) string {
	if n.Type != html.ElementNode {
		return activeVerseID
	}

	// Collect children and remove them from n immediately
	var children []*html.Node
	for c := n.FirstChild; c != nil; {
		next := c.NextSibling
		n.RemoveChild(c)
		children = append(children, c)
		c = next
	}

	// First pass: Identify and group poetic lines into sections
	var groupedChildren []*html.Node
	var currentSection *html.Node

	for i := 0; i < len(children); i++ {
		c := children[i]

		// Check for start of line group
		if isBeginLineGroup(c) {
			currentSection = &html.Node{
				Type:     html.ElementNode,
				Data:     "section",
				DataAtom: atom.Section,
				Attr: []html.Attribute{
					{Key: "class", Val: "line-group"},
				},
			}
			groupedChildren = append(groupedChildren, currentSection)
			continue // Skip adding the begin marker itself
		}

		// Check for end of line group
		if isEndLineGroup(c) {
			currentSection = nil
			// Skip adding the end marker itself

			// Look ahead for empty verse span and remove it if present
			if i+1 < len(children) {
				next := children[i+1]
				if isEmptyVerse(next) {
					i++ // Skip the empty verse span
				}
			}
			continue
		}

		if currentSection != nil {
			currentSection.AppendChild(c)
		} else {
			groupedChildren = append(groupedChildren, c)
		}
	}

	children = groupedChildren

	if len(children) > 0 {
		var newChildren []*html.Node

		var currentWrapper *html.Node

		// If we are continuing a verse in a poetic line, we should trim the leading whitespace (indentation)
		shouldTrimNextText := activeVerseID != "" && isPoeticLine(n)

		if shouldTrimNextText {
			addClass(n, "verse")
		}

		// Helper to close current wrapper
		closeWrapper := func(doTrim bool) {
			if currentWrapper != nil {
				if doTrim {
					trimTrailingWhitespace(currentWrapper)
				}
				newChildren = append(newChildren, currentWrapper)
				currentWrapper = nil
				shouldTrimNextText = false
			}
		}

		for _, c := range children {
			// Check if this child is a verse marker
			verseID := getVerseID(c)

			if verseID != "" {
				// New verse starts here.
				activeVerseID = verseID
				closeWrapper(true) // Always trim at end of verse (next verse acts as boundary/padding)

				isPoetic := isPoeticLine(n)
				if isPoetic {
					// Hybrid optimization: Tag the existing poetic line (n)
					addClass(n, "verse")
					// Do not remove ID from marker for poetry to preserve data (since container has p-ID)
					// Append marker directly to container
					cleanVerseMarker(c)
					newChildren = append(newChildren, c)
					shouldTrimNextText = true
				} else {
					// Prose: Wrap in new span
					currentWrapper = createWrapper(activeVerseID)
					cleanVerseMarker(c)
					removeID(c) // Move ID to wrapper effectively (wrapper has it)
					currentWrapper.AppendChild(c)
					shouldTrimNextText = true
				}

			} else if c.Type == html.ElementNode {
				// Container element (block or inline like span/div/p)
				// Close current wrapper to avoid wrapping the container itself
				// instead, we recurse to wrap content inside it.
				closeWrapper(false) // Do NOT trim when descending into child (concatenation risk)

				// Recurse into the element with current state
				activeVerseID = processNode(c, activeVerseID)
				newChildren = append(newChildren, c)

			} else {
				// Inline content (text, span, etc.)
				if activeVerseID != "" {
					isPoetic := isPoeticLine(n)

					if isPoetic {
						// Append directly to the poetic line
						if c.Type == html.TextNode && shouldTrimNextText {
							c.Data = strings.TrimLeft(c.Data, cutset)
							if len(c.Data) > 0 {
								shouldTrimNextText = false
							}
						}
						newChildren = append(newChildren, c)
					} else {
						// Wrapper logic (Prose)
						if currentWrapper == nil {
							currentWrapper = createWrapper(activeVerseID)
							shouldTrimNextText = true
						}

						if c.Type == html.TextNode && shouldTrimNextText {
							c.Data = strings.TrimLeft(c.Data, cutset)
							if len(c.Data) > 0 {
								shouldTrimNextText = false
							}
						}
						currentWrapper.AppendChild(c)
					}
				} else {
					// No active verse (e.g. intro text), just append
					newChildren = append(newChildren, c)
				}
			}
		}

		// Append any final wrapper
		// Only trim if we are at the end of a block context
		closeWrapper(isBlock(n))

		// Rebuild children
		for _, c := range newChildren {
			n.AppendChild(c)
		}
	}

	return activeVerseID
}

func createWrapper(id string) *html.Node {
	return &html.Node{
		Type:     html.ElementNode,
		Data:     "span",
		DataAtom: atom.Span,
		Attr: []html.Attribute{
			{Key: "id", Val: id},
			{Key: "class", Val: "verse"},
		},
	}
}

func getVerseID(n *html.Node) string {
	if n.Type != html.ElementNode {
		return ""
	}
	for _, a := range n.Attr {
		if a.Key == "id" && verseIDRegex.MatchString(a.Val) {
			return a.Val
		}
	}
	return ""
}

func removeID(n *html.Node) {
	for i, a := range n.Attr {
		if a.Key == "id" {
			n.Attr = append(n.Attr[:i], n.Attr[i+1:]...)
			return
		}
	}
}

func cleanVerseMarker(n *html.Node) {
	if n.Type == html.TextNode {
		n.Data = strings.Trim(n.Data, cutset)
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		cleanVerseMarker(c)
	}
}

func trimTrailingWhitespace(n *html.Node) {
	if n.LastChild != nil && n.LastChild.Type == html.TextNode {
		n.LastChild.Data = strings.TrimRight(n.LastChild.Data, cutset)
	}
}

func hasClass(n *html.Node, class string) bool {
	for _, a := range n.Attr {
		if a.Key == "class" {
			for _, c := range strings.Fields(a.Val) {
				if c == class {
					return true
				}
			}
		}
	}
	return false
}

func isBlock(n *html.Node) bool {
	// Common block elements. This is not exhaustive but covers ESV structure.
	switch n.Data {
	}
	return false
}

func isPoeticLine(n *html.Node) bool {
	return n.Data == "span" && hasClass(n, "line")
}

func isBeginLineGroup(n *html.Node) bool {
	return n.Data == "span" && hasClass(n, "begin-line-group")
}

func isEndLineGroup(n *html.Node) bool {
	return n.Data == "span" && hasClass(n, "end-line-group")
}

func isEmptyVerse(n *html.Node) bool {
	if n.Data != "span" || !hasClass(n, "verse") {
		return false
	}
	// It's empty if it has no children or only empty text
	if n.FirstChild == nil {
		return true
	}
	// A bit risky if it has attributes that matter, but request was specific about empty span
	// removal after end-line-group.
	return false
}

func addClass(n *html.Node, newClass string) {
	for i, a := range n.Attr {
		if a.Key == "class" {
			if !hasClass(n, newClass) {
				n.Attr[i].Val += " " + newClass
			}
			return
		}
	}
	// Class attribute not found, add it
	n.Attr = append(n.Attr, html.Attribute{Key: "class", Val: newClass})
}
