package apibible

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

// ProcessPassageHTML transforms API.Bible passage HTML to match Daily Soaps uniform scripture styling,
// wrapping each verse in <span class="verse" data-ref="BBCCCVVV"> and marking numbers as <b class="verse-num">.
func ProcessPassageHTML(htmlStr string, defaultBookNum int, defaultChapter int) (string, error) {
	if htmlStr == "" {
		return "", nil
	}

	nodes, err := html.ParseFragment(strings.NewReader(htmlStr), &html.Node{
		Type:     html.ElementNode,
		Data:     "body",
		DataAtom: atom.Body,
	})
	if err != nil {
		return "", fmt.Errorf("failed to parse API.Bible HTML fragment: %w", err)
	}

	var buf bytes.Buffer
	state := &transformState{
		bookNum:    defaultBookNum,
		chapterNum: defaultChapter,
	}

	for _, node := range nodes {
		processNode(node, state)
		if err := html.Render(&buf, node); err != nil {
			return "", fmt.Errorf("failed to render node: %w", err)
		}
	}

	return buf.String(), nil
}

type transformState struct {
	bookNum        int
	chapterNum     int
	activeVerseRef string
}

func processNode(n *html.Node, state *transformState) {
	if n.Type != html.ElementNode {
		return
	}

	// Detect chapter changes if present in data-sid or attributes (e.g. data-sid="GEN 2")
	if sid := getAttr(n, "data-sid"); sid != "" {
		parts := strings.Split(sid, " ")
		if len(parts) >= 2 {
			if b, ok := USFMToBookInfo(parts[0]); ok {
				state.bookNum = b.Number
			}
			cv := strings.Split(parts[1], ":")
			if c, err := strconv.Atoi(cv[0]); err == nil {
				state.chapterNum = c
			}
		}
	}

	// Extract children to process
	var children []*html.Node
	for c := n.FirstChild; c != nil; {
		next := c.NextSibling
		n.RemoveChild(c)
		children = append(children, c)
		c = next
	}

	var newChildren []*html.Node
	var currentWrapper *html.Node

	closeWrapper := func() {
		if currentWrapper != nil {
			if currentWrapper.FirstChild != nil {
				newChildren = append(newChildren, currentWrapper)
			}
			currentWrapper = nil
		}
	}

	for _, c := range children {
		if isVerseMarker(c) {
			closeWrapper()

			vNum := extractVerseNum(c)
			if vNum > 0 && state.bookNum > 0 && state.chapterNum > 0 {
				state.activeVerseRef = fmt.Sprintf("%02d%03d%03d", state.bookNum, state.chapterNum, vNum)
			}

			// Format marker as <b class="verse-num">vNum</b>
			marker := createVerseMarker(vNum)
			currentWrapper = createVerseWrapper(state.activeVerseRef)
			currentWrapper.AppendChild(marker)
			continue
		}

		if c.Type == html.ElementNode {
			if isBlockElement(c) {
				closeWrapper()
				processNode(c, state)
				newChildren = append(newChildren, c)
				continue
			}

			// Inline element
			processNode(c, state)
			if state.activeVerseRef != "" {
				if currentWrapper == nil {
					currentWrapper = createVerseWrapper(state.activeVerseRef)
				}
				currentWrapper.AppendChild(c)
			} else {
				newChildren = append(newChildren, c)
			}
			continue
		}

		if c.Type == html.TextNode {
			if state.activeVerseRef != "" {
				if currentWrapper == nil {
					currentWrapper = createVerseWrapper(state.activeVerseRef)
				}
				currentWrapper.AppendChild(c)
			} else {
				newChildren = append(newChildren, c)
			}
			continue
		}

		newChildren = append(newChildren, c)
	}

	closeWrapper()

	for _, child := range newChildren {
		n.AppendChild(child)
	}
}

func isVerseMarker(n *html.Node) bool {
	if n.Type != html.ElementNode {
		return false
	}
	if hasClass(n, "v") {
		return true
	}
	if getAttr(n, "data-number") != "" && n.DataAtom == atom.Span {
		return true
	}
	return false
}

func extractVerseNum(n *html.Node) int {
	if numStr := getAttr(n, "data-number"); numStr != "" {
		// May have suffixes like "1a"
		numStr = regexp.MustCompile(`^\d+`).FindString(numStr)
		if val, err := strconv.Atoi(numStr); err == nil {
			return val
		}
	}
	// Try reading text inside node
	var buf strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.TextNode {
			buf.WriteString(c.Data)
		}
	}
	digits := regexp.MustCompile(`\d+`).FindString(buf.String())
	if val, err := strconv.Atoi(digits); err == nil {
		return val
	}
	return 0
}

func createVerseMarker(vNum int) *html.Node {
	marker := &html.Node{
		Type:     html.ElementNode,
		Data:     "b",
		DataAtom: atom.B,
		Attr: []html.Attribute{
			{Key: "class", Val: "verse-num"},
		},
	}
	text := &html.Node{
		Type: html.TextNode,
		Data: strconv.Itoa(vNum),
	}
	marker.AppendChild(text)
	return marker
}

func createVerseWrapper(ref string) *html.Node {
	wrapper := &html.Node{
		Type:     html.ElementNode,
		Data:     "span",
		DataAtom: atom.Span,
		Attr: []html.Attribute{
			{Key: "class", Val: "verse"},
		},
	}
	if ref != "" {
		wrapper.Attr = append(wrapper.Attr, html.Attribute{Key: "data-ref", Val: ref})
	}
	return wrapper
}

func isBlockElement(n *html.Node) bool {
	switch n.DataAtom {
	case atom.P, atom.Div, atom.Section, atom.Article, atom.H1, atom.H2, atom.H3, atom.H4, atom.H5, atom.H6, atom.Table, atom.Blockquote:
		return true
	default:
		return false
	}
}

func getAttr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func hasClass(n *html.Node, className string) bool {
	classVal := getAttr(n, "class")
	classes := strings.Fields(classVal)
	return slices.Contains(classes, className)
}
