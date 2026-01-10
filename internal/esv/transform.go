package esv

import (
	"bytes"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

const (
	// cutset includes spaces, newlines, tabs, and non-breaking spaces (\u00A0).
	cutset = " \t\n\r\u00A0"
)

// processPassageHTML takes an HTML string containing verses and wraps/tags each verse
// (highlight + following text) in a span that carries the verse ID or tags the existing span.
func processPassageHTML(htmlStr string) (string, error) {
	// Encode escaped unicode characters into actual characters, e.g. \u2013 -> –)
	htmlStr = unescapeString(htmlStr)

	// Parse the HTML fragment.
	nodes, err := html.ParseFragment(strings.NewReader(htmlStr), &html.Node{
		Type:     html.ElementNode,
		Data:     "body",
		DataAtom: atom.Body,
	})

	if err != nil {
		return "", fmt.Errorf("failed to parse HTML fragment: %w", err)
	}

	var buf bytes.Buffer
	var activeVerseRef string // The 8-digit verse reference.

	// Walk the DOM tree to clean up HTML and wrap verses.
	for _, node := range nodes {
		// Filter top-level end-line-group (unlikely but safe).
		if isEndLineGroup(node) {
			continue
		}

		activeVerseRef = processNode(node, activeVerseRef)

		// Unwrap P containing Section
		if node.DataAtom == atom.P && hasSection(node) {
			for c := node.FirstChild; c != nil; c = c.NextSibling {
				if err := html.Render(&buf, c); err != nil {
					return "", fmt.Errorf("failed to render child node: %w", err)
				}
			}
			continue
		}

		// Filter empty P (<p></p>).
		if node.DataAtom == atom.P && node.FirstChild == nil {
			continue
		}

		if err := html.Render(&buf, node); err != nil {
			return "", fmt.Errorf("failed to render node: %w", err)
		}
	}

	return buf.String(), nil
}

func unescapeString(s string) string {
	unescapeRegex := regexp.MustCompile(`\\u([0-9a-fA-F]{4})`)

	return unescapeRegex.ReplaceAllStringFunc(s, func(match string) string {
		code := match[2:]
		val, err := strconv.ParseInt(code, 16, 32)
		if err != nil {
			return match
		}
		return string(rune(val))
	})
}

// processNode recursively traverses the DOM tree and transforms it.
// Returns the updated activeVerseRef.
func processNode(n *html.Node, activeVerseRef string) string {
	if n.Type != html.ElementNode {
		return activeVerseRef
	}

	// Clean up attributes for this node (parent).
	cleanupAttributes(n)

	// Extract children to process them.
	var children []*html.Node
	for c := n.FirstChild; c != nil; {
		next := c.NextSibling
		n.RemoveChild(c)
		children = append(children, c)
		c = next
	}

	// Early trim for verse markers.
	if n.DataAtom == atom.B && hasClass(n, "verse-num") {
		for _, c := range children {
			if c.Type == html.TextNode {
				c.Data = strings.Trim(c.Data, cutset)
			}
		}
	}

	// Use the extracted children to check for Copyright to abort verse context for this block.
	if slices.ContainsFunc(children, isCopyrightLink) {
		activeVerseRef = ""
	}

	// Step 1: Handle line grouping (convert begin/end spans to section).
	var groupedChildren []*html.Node
	var currentSection *html.Node

	for i := 0; i < len(children); i++ {
		c := children[i]
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
			continue
		}
		if isEndLineGroup(c) {
			currentSection = nil
			continue
		}
		if currentSection != nil {
			currentSection.AppendChild(c)
		} else {
			groupedChildren = append(groupedChildren, c)
		}
	}
	children = groupedChildren

	// Step 2: Traverse children, wrap verses, and clean content
	var newChildren []*html.Node
	var currentWrapper *html.Node
	var shouldTrimLeading bool

	// Helper to close current wrapper
	closeWrapper := func() {
		if currentWrapper != nil {
			trimTrailingWhitespace(currentWrapper)
			if currentWrapper.FirstChild != nil {
				newChildren = append(newChildren, currentWrapper)
			}
			currentWrapper = nil
		}
	}

	for i := 0; i < len(children); i++ {
		c := children[i]

		// Check if this child is a verse marker
		// Note: we must check this BEFORE cleanupAttributes removes the ID
		ref := getVerseRef(c)

		// Clean child attributes if element
		if c.Type == html.ElementNode {
			cleanupAttributes(c)
		}

		if ref != "" {
			// Start of a new verse
			closeWrapper()
			activeVerseRef = ref
			shouldTrimLeading = true

			cleanVerseMarker(c)

			if isStylizedLine(n) {
				newChildren = append(newChildren, c)
			} else {
				currentWrapper = createVerseWrapper(activeVerseRef)
				currentWrapper.AppendChild(c)
			}
			continue
		}

		// Copyright link
		if isCopyrightLink(c) {
			closeWrapper()
			activeVerseRef = ""
			newChildren = append(newChildren, c)
			continue
		}

		if c.Type == html.ElementNode {
			// Block or 'section' breaks wrapper
			if isBlock(c) {
				closeWrapper()
				activeVerseRef = processNode(c, activeVerseRef)

				// Unwrap P containing Section
				if c.DataAtom == atom.P && hasSection(c) {
					for pc := c.FirstChild; pc != nil; pc = pc.NextSibling {
						newChildren = append(newChildren, pc)
					}
					shouldTrimLeading = false
					continue
				}

				// Filter empty P inside blocks
				if c.DataAtom == atom.P && c.FirstChild == nil {
					shouldTrimLeading = false
					continue
				}

				newChildren = append(newChildren, c)
				// Reset local trim state for block?
				shouldTrimLeading = false
				continue
			}

			// BR handling
			if c.DataAtom == atom.Br {
				closeWrapper()
				newChildren = append(newChildren, c)
				shouldTrimLeading = false
				continue
			}

			// Stylized line
			if isStylizedLine(c) {
				closeWrapper()
				activeVerseRef = processNode(c, activeVerseRef)
				if activeVerseRef != "" {
					addClass(c, "verse")
					setAttr(c, "data-ref", activeVerseRef)
				}
				newChildren = append(newChildren, c)
				shouldTrimLeading = false
				continue
			}

			// Inline element
			// If we enter an inline element, we should reset leading trim
			// because unless we pass it down, we can't enforce it inside.
			shouldTrimLeading = false

			if activeVerseRef != "" && !isStylizedLine(n) {
				if currentWrapper == nil {
					currentWrapper = createVerseWrapper(activeVerseRef)
				}
				processNode(c, activeVerseRef)

				// Filter empty P
				if c.DataAtom == atom.P && c.FirstChild == nil {
					continue
				}

				currentWrapper.AppendChild(c)
			} else {
				processNode(c, activeVerseRef)

				// Filter empty P
				if c.DataAtom == atom.P && c.FirstChild == nil {
					continue
				}

				newChildren = append(newChildren, c)
			}

		} else if c.Type == html.TextNode {
			if shouldTrimLeading {
				c.Data = strings.TrimLeft(c.Data, cutset)
				if len(c.Data) > 0 {
					shouldTrimLeading = false
				}
			}

			cleanTextNode(c)

			if strings.TrimSpace(c.Data) == "" {
				// Pure whitespace
				if n.DataAtom == atom.Section && hasClass(n, "line-group") {
					newChildren = append(newChildren, c)
					continue
				}
			}

			if activeVerseRef != "" {
				if isStylizedLine(n) {
					newChildren = append(newChildren, c)
				} else {
					if currentWrapper == nil {
						currentWrapper = createVerseWrapper(activeVerseRef)
					}
					currentWrapper.AppendChild(c)
				}
			} else {
				newChildren = append(newChildren, c)
			}
		}
	}
	closeWrapper()

	// Re-attach children
	for _, c := range newChildren {
		n.AppendChild(c)
	}

	// Post-processing for n
	if n.Type == html.ElementNode {
		if n.DataAtom == atom.P || n.DataAtom == atom.Span {
			trimChildrenWhitespace(n)
		}
	}

	return activeVerseRef
}

// createVerseWrapper creates a span element to wrap each verse or verse line.
// The span is given a class of "verse" and a data-ref attribute with the verse reference.
func createVerseWrapper(ref string) *html.Node {
	return &html.Node{
		Type:     html.ElementNode,
		Data:     "span",
		DataAtom: atom.Span,
		Attr: []html.Attribute{
			{Key: "class", Val: "verse"},
			{Key: "data-ref", Val: ref},
		},
	}
}

// cleanupAttributes removes element attributes that are not needed for our use case.
func cleanupAttributes(n *html.Node) {
	switch n.DataAtom {
	case atom.P:
		removeAttr(n, "id")
		removeClass(n, "virtual")
	case atom.Span:
		if hasClass(n, "line") {
			removeAttr(n, "id")
		}
	case atom.B:
		removeAttr(n, "id")
	case atom.H1, atom.H2, atom.H3, atom.H4, atom.H5, atom.H6:
		removeAttr(n, "id")
	}
}

func getVerseRef(n *html.Node) string {
	if n.Type != html.ElementNode {
		return ""
	}

	// verseIDRegex matches verse IDs like "v01002017-1" and captures the ref "01002017"
	verseIDRegex := regexp.MustCompile(`^v(\d{8}).*`)

	for _, a := range n.Attr {
		if a.Key == "id" {
			matches := verseIDRegex.FindStringSubmatch(a.Val)
			if len(matches) > 1 {
				return matches[1]
			}
		}
	}
	return ""
}

func cleanVerseMarker(n *html.Node) {
	if n.Type == html.ElementNode && n.DataAtom == atom.B && hasClass(n, "verse-num") {
		trimChildrenWhitespace(n)
	}
}

func cleanTextNode(n *html.Node) {

}

func trimChildrenWhitespace(n *html.Node) {
	if n.FirstChild != nil && n.FirstChild.Type == html.TextNode {
		n.FirstChild.Data = strings.TrimLeft(n.FirstChild.Data, cutset)
		if len(n.FirstChild.Data) == 0 {
			n.RemoveChild(n.FirstChild)
			if n.FirstChild != nil {
				trimChildrenWhitespace(n)
			}
		}
	}
	if n.LastChild != nil && n.LastChild.Type == html.TextNode {
		n.LastChild.Data = strings.TrimRight(n.LastChild.Data, cutset)
		if len(n.LastChild.Data) == 0 {
			n.RemoveChild(n.LastChild)
			if n.LastChild != nil {
				trimChildrenWhitespace(n)
			}
		}
	}
}

func trimTrailingWhitespace(n *html.Node) {
	if n.LastChild != nil && n.LastChild.Type == html.TextNode {
		n.LastChild.Data = strings.TrimRight(n.LastChild.Data, cutset)
		if len(n.LastChild.Data) == 0 {
			n.RemoveChild(n.LastChild)
		}
	}
}

func hasClass(n *html.Node, class string) bool {
	for _, a := range n.Attr {
		if a.Key == "class" {
			if slices.Contains(strings.Fields(a.Val), class) {
				return true
			}
		}
	}
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
	n.Attr = append(n.Attr, html.Attribute{Key: "class", Val: newClass})
}

func removeClass(n *html.Node, target string) {
	for i, a := range n.Attr {
		if a.Key == "class" {
			fields := strings.Fields(a.Val)
			var newFields []string
			changed := false
			for _, f := range fields {
				if f == target {
					changed = true
					continue
				}
				newFields = append(newFields, f)
			}
			if changed {
				if len(newFields) == 0 {
					n.Attr = append(n.Attr[:i], n.Attr[i+1:]...)
				} else {
					n.Attr[i].Val = strings.Join(newFields, " ")
				}
			}
			return
		}
	}
}

// setAttr sets an attribute on a node, creating it if it doesn't exist.
func setAttr(n *html.Node, key, val string) {
	for i, a := range n.Attr {
		if a.Key == key {
			n.Attr[i].Val = val
			return
		}
	}
	n.Attr = append(n.Attr, html.Attribute{Key: key, Val: val})
}

// removeAttr removes an attribute from a node.
func removeAttr(n *html.Node, key string) {
	for i, a := range n.Attr {
		if a.Key == key {
			n.Attr = append(n.Attr[:i], n.Attr[i+1:]...)
			return
		}
	}
}

// isBlock returns true if the node is a block element.
// Technically with HTML5 there is no such thing as a block element.
func isBlock(n *html.Node) bool {
	return n.DataAtom == atom.P || n.DataAtom == atom.Div || n.DataAtom == atom.Section ||
		n.DataAtom == atom.H1 || n.DataAtom == atom.H2 || n.DataAtom == atom.H3 ||
		n.DataAtom == atom.H4 || n.DataAtom == atom.H5 || n.DataAtom == atom.H6
}

// isStylizedLine returns true if the node is a poetic line.
// A stylized line is a span element with a class of "line".
// It is used in places where the text is stylized to have indented after the initial line.
// This is common in the Psalms and in quotes.
func isStylizedLine(n *html.Node) bool {
	return n.DataAtom == atom.Span && hasClass(n, "line")
}

// isBeginLineGroup returns true if the node is a begin line group.
// These span elements are part of the raw response from the ESV API.
func isBeginLineGroup(n *html.Node) bool {
	return n.DataAtom == atom.Span && hasClass(n, "begin-line-group")
}

// isEndLineGroup returns true if the node is an end line group.
// These span elements are part of the raw response from the ESV API.
func isEndLineGroup(n *html.Node) bool {
	return n.DataAtom == atom.Span && hasClass(n, "end-line-group")
}

// isCopyrightLink returns true if the node is a copyright link.
func isCopyrightLink(n *html.Node) bool {
	return n.DataAtom == atom.A && hasClass(n, "copyright")
}

// hasSection returns true if the node or any of its descendants is a section element.
func hasSection(n *html.Node) bool {
	for c := range n.Descendants() {
		if c.Type == html.ElementNode && c.DataAtom == atom.Section {
			return true
		}
	}
	return false
}
