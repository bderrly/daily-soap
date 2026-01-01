package esv

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// processPassageHTML takes an HTML string containing verses and wraps each verse
// (highlight + following text) in a span that carries the verse ID.
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

	if len(children) > 0 {
		var newChildren []*html.Node
		var currentWrapper *html.Node

		// Helper to close current wrapper
		closeWrapper := func(doTrim bool) {
			if currentWrapper != nil {
				if doTrim {
					trimTrailingWhitespace(currentWrapper)
				}
				newChildren = append(newChildren, currentWrapper)
				currentWrapper = nil
			}
		}

		for _, c := range children {
			// Check if this child is a verse marker
			verseID := getVerseID(c)

			if verseID != "" {
				// New verse starts here.
				activeVerseID = verseID
				closeWrapper(true) // Always trim at end of verse (next verse acts as boundary/padding)

				// Start new wrapper
				currentWrapper = createWrapper(activeVerseID)

				// FIX: Insert &nbsp; before text in verse number element
				if c.Data == "b" || hasClass(c, "verse-num") {
					insertNBSP(c)
				}

				// Strip ID from marker
				removeID(c)

				// Add marker to wrapper
				currentWrapper.AppendChild(c)

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
					// We have an active verse context
					if currentWrapper == nil {
						currentWrapper = createWrapper(activeVerseID)
					}
					currentWrapper.AppendChild(c)
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
			{Key: "class", Val: "verse-span"},
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

func insertNBSP(n *html.Node) {
	nbsp := &html.Node{
		Type:     html.TextNode,
		Data:     "\u00A0",
		DataAtom: 0,
	}
	nbsp.Parent = n
	if n.FirstChild != nil {
		nbsp.NextSibling = n.FirstChild
		n.FirstChild.PrevSibling = nbsp
	}
	n.FirstChild = nbsp
}

func trimTrailingWhitespace(n *html.Node) {
	if n.LastChild != nil && n.LastChild.Type == html.TextNode {
		n.LastChild.Data = strings.TrimRight(n.LastChild.Data, " \t\n\r")
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
	case "p", "div", "blockquote", "h1", "h2", "h3", "h4", "h5", "h6", "ul", "ol", "li", "body", "br":
		return true
	}
	return false
}
