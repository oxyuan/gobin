package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// Structure to hold link information
type linkInfo struct {
	text string
	url  string
}

// Regex to find markdown links: [text](url)
var markdownLinkRegex = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)

// Regex for sanitizing filenames (replace non-alphanumeric with underscore)
var filenameSanitizeRegex = regexp.MustCompile(`[^a-zA-Z0-9_-]+`)

func main() {
	inputMarkdown := `
### **JVM 相关命令**
- [dashboard](https://arthas.aliyun.com/doc/dashboard.html)  
- [getstatic](https://arthas.aliyun.com/doc/getstatic.html)  
- [heapdump](https://arthas.aliyun.com/doc/heapdump.html)  
- [jvm](https://arthas.aliyun.com/doc/jvm.html)  
- [logger](https://arthas.aliyun.com/doc/logger.html)  
- [mbean](https://arthas.aliyun.com/doc/mbean.html)  
- [memory](https://arthas.aliyun.com/doc/memory.html)  
- [ognl](https://arthas.aliyun.com/doc/ognl.html)  
- [perfcounter](https://arthas.aliyun.com/doc/perfcounter.html)  
- [sysenv](https://arthas.aliyun.com/doc/sysenv.html)  
- [sysprop](https://arthas.aliyun.com/doc/sysprop.html)  
- [thread](https://arthas.aliyun.com/doc/thread.html)  
- [vmoption](https://arthas.aliyun.com/doc/vmoption.html)  
- [vmtool](https://arthas.aliyun.com/doc/vmtool.html)  

---

### **Class/ClassLoader 相关命令**
- [classloader](https://arthas.aliyun.com/doc/classloader.html)  
- [dump](https://arthas.aliyun.com/doc/dump.html)  
- [jad](https://arthas.aliyun.com/doc/jad.html)  
- [mc](https://arthas.aliyun.com/doc/mc.html)  
- [redefine](https://arthas.aliyun.com/doc/redefine.html)  
- [retransform](https://arthas.aliyun.com/doc/retransform.html)  
- [sc](https://arthas.aliyun.com/doc/sc.html)  
- [sm](https://arthas.aliyun.com/doc/sm.html)  

---

### **监控/观测/跟踪命令**
- [monitor](https://arthas.aliyun.com/doc/monitor.html)  
- [stack](https://arthas.aliyun.com/doc/stack.html)  
- [trace](https://arthas.aliyun.com/doc/trace.html)  
- [tt](https://arthas.aliyun.com/doc/tt.html)  
- [watch](https://arthas.aliyun.com/doc/watch.html)  

---

### **Profiler/火焰图**
- [profiler](https://arthas.aliyun.com/doc/profiler.html)  
- [jfr](https://arthas.aliyun.com/doc/jfr.html)  

---

### **鉴权**
- [auth](https://arthas.aliyun.com/doc/auth.html)  

---

### **Options**
- [options](https://arthas.aliyun.com/doc/options.html)  

---

### **管道与数据处理**
- [grep](https://arthas.aliyun.com/doc/grep.html)  
- [plaintext](https://arthas.aliyun.com/doc/plaintext.html)  
- [wc](https://arthas.aliyun.com/doc/wc.html)  

---

### **后台异步任务**
- [jobs](https://arthas.aliyun.com/doc/jobs.html)  
- [kill](https://arthas.aliyun.com/doc/kill.html)  
- [fg](https://arthas.aliyun.com/doc/fg.html)  
- [bg](https://arthas.aliyun.com/doc/bg.html)  

---

### **基础命令**
- [base64](https://arthas.aliyun.com/doc/base64.html)  
- [cat](https://arthas.aliyun.com/doc/cat.html)  
- [cls](https://arthas.aliyun.com/doc/cls.html)  
- [echo](https://arthas.aliyun.com/doc/echo.html)  
- [help](https://arthas.aliyun.com/doc/help.html)  
- [history](https://arthas.aliyun.com/doc/history.html)  
- [keymap](https://arthas.aliyun.com/doc/keymap.html)  
- [pwd](https://arthas.aliyun.com/doc/pwd.html)  
- [quit](https://arthas.aliyun.com/doc/quit.html)  
- [reset](https://arthas.aliyun.com/doc/reset.html)  
- [session](https://arthas.aliyun.com/doc/session.html)  
- [stop](https://arthas.aliyun.com/doc/stop.html)  
- [tee](https://arthas.aliyun.com/doc/tee.html)  
- [version](https://arthas.aliyun.com/doc/version.html)
`

	links := parseLinks(inputMarkdown)
	outputDir := "arthas_docs_md" // Directory to save markdown files

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		log.Fatalf("Failed to create output directory '%s': %v", outputDir, err)
	}

	var wg sync.WaitGroup
	client := &http.Client{} // Use a shared client

	for _, link := range links {
		if link.url == "" || link.text == "" {
			continue
		}
		wg.Add(1)
		// Launch goroutine for each link
		go func(l linkInfo) {
			defer wg.Done()
			fmt.Printf("Processing: %s (%s)\n", l.text, l.url)

			req, err := http.NewRequest("GET", l.url, nil)
			if err != nil {
				log.Printf("Error creating request for %s: %v", l.url, err)
				return
			}
			// Set a user agent to mimic a browser, some sites might require it
			req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")

			resp, err := client.Do(req)
			if err != nil {
				log.Printf("Error fetching %s: %v", l.url, err)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				log.Printf("Error fetching %s: Status %s", l.url, resp.Status)
				return
			}

			bodyBytes, err := io.ReadAll(resp.Body)
			if err != nil {
				log.Printf("Error reading body for %s: %v", l.url, err)
				return
			}

			doc, err := html.Parse(bytes.NewReader(bodyBytes))
			if err != nil {
				log.Printf("Error parsing HTML for %s: %v", err, l.url)
				return
			}

			// Find the main content node (<main class="page">)
			mainNode := findMainContentNode(doc)
			if mainNode == nil {
				log.Printf("Could not find main content node (<main class=\"page\">) in %s", l.url)
				// Fallback: try to convert the whole body if main not found
				bodyNode := findBodyNode(doc)
				if bodyNode != nil {
					mainNode = bodyNode
					log.Printf("Warning: Falling back to converting entire <body> for %s", l.url)
				} else {
					log.Printf("Error: Could not find <body> node either for %s", l.url)
					return
				}
			}

			// Convert the main node content to Markdown
			markdownContent := htmlNodeToMarkdown(mainNode)

			// Sanitize filename
			safeFilename := filenameSanitizeRegex.ReplaceAllString(l.text, "_")
			outputFilename := filepath.Join(outputDir, safeFilename+".md")

			// Write the markdown to file
			err = os.WriteFile(outputFilename, []byte(markdownContent), 0644)
			if err != nil {
				log.Printf("Error writing file %s: %v", outputFilename, err)
			} else {
				fmt.Printf("Successfully wrote: %s\n", outputFilename)
			}

		}(link)
	}

	wg.Wait() // Wait for all goroutines to finish
	fmt.Println("\nProcessing complete.")
}

// Parses markdown text to extract [text](url) links
func parseLinks(markdown string) []linkInfo {
	matches := markdownLinkRegex.FindAllStringSubmatch(markdown, -1)
	links := make([]linkInfo, 0, len(matches))
	for _, match := range matches {
		if len(match) == 3 {
			links = append(links, linkInfo{text: strings.TrimSpace(match[1]), url: strings.TrimSpace(match[2])})
		}
	}
	return links
}

// Recursively finds the first node matching tag and class
func findNodeByTagAndClass(n *html.Node, tag atom.Atom, className string) *html.Node {
	if n.Type == html.ElementNode && n.DataAtom == tag {
		for _, attr := range n.Attr {
			if attr.Key == "class" {
				classes := strings.Fields(attr.Val)
				for _, c := range classes {
					if c == className {
						return n
					}
				}
			}
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if found := findNodeByTagAndClass(c, tag, className); found != nil {
			return found
		}
	}
	return nil
}

// Finds the main content node (<main class="page">)
func findMainContentNode(doc *html.Node) *html.Node {
	return findNodeByTagAndClass(doc, atom.Main, "page")
}

// Finds the body node (<body>)
func findBodyNode(doc *html.Node) *html.Node {
	var body *html.Node
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && n.DataAtom == atom.Body {
			body = n
			return // Found body, stop searching
		}
		// Traverse children only if body not found yet
		for c := n.FirstChild; c != nil && body == nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)
	return body
}

// Extracts the href attribute from a node
func getHref(n *html.Node) string {
	for _, attr := range n.Attr {
		if attr.Key == "href" {
			return attr.Val
		}
	}
	return ""
}

// Extracts text content from a node and its children
func getNodeText(n *html.Node) string {
	var buf bytes.Buffer
	var f func(*html.Node)
	f = func(node *html.Node) {
		if node.Type == html.TextNode {
			buf.WriteString(node.Data)
		}
		// Only recurse for non-code elements to preserve whitespace in code
		if node.Type == html.ElementNode && node.DataAtom != atom.Code && node.DataAtom != atom.Pre {
			for c := node.FirstChild; c != nil; c = c.NextSibling {
				f(c)
			}
		} else if node.Type == html.ElementNode && (node.DataAtom == atom.Code || node.DataAtom == atom.Pre) {
			for c := node.FirstChild; c != nil; c = c.NextSibling {
				if c.Type == html.TextNode {
					buf.WriteString(c.Data) // Get raw text from code/pre children
				} else {
					f(c) // Recurse for potential nested elements within code? (Unlikely but possible)
				}
			}
		}
	}
	f(n)
	return buf.String()
}

// Converts an HTML node and its children to Markdown recursively
func htmlNodeToMarkdown(n *html.Node) string {
	if n == nil {
		return ""
	}

	var buf bytes.Buffer

	switch n.Type {
	case html.TextNode:
		// Basic whitespace cleanup for general text
		// Avoid trimming if inside pre/code
		parentTag := atom.A // Default to something that allows trimming
		if n.Parent != nil {
			parentTag = n.Parent.DataAtom
		}
		if parentTag == atom.Pre || parentTag == atom.Code {
			return n.Data // Preserve whitespace
		}
		// Trim leading/trailing space, replace multiple spaces with one
		trimmed := strings.TrimSpace(n.Data)
		processed := strings.Join(strings.Fields(trimmed), " ")
		// Add back a single space if the original text had surrounding spaces
		// (heuristic to avoid words sticking together)
		if strings.HasPrefix(n.Data, " ") && !strings.HasPrefix(processed, " ") {
			processed = " " + processed
		}
		if strings.HasSuffix(n.Data, " ") && !strings.HasSuffix(processed, " ") {
			processed = processed + " "
		}

		return processed

	case html.ElementNode:
		tagName := n.DataAtom
		// Recursively get markdown for children first
		var childrenMarkdown strings.Builder
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			childrenMarkdown.WriteString(htmlNodeToMarkdown(c))
		}
		childMd := childrenMarkdown.String()

		switch tagName {
		case atom.H1:
			buf.WriteString("# " + strings.TrimSpace(childMd) + "\n\n")
		case atom.H2:
			buf.WriteString("## " + strings.TrimSpace(childMd) + "\n\n")
		case atom.H3:
			buf.WriteString("### " + strings.TrimSpace(childMd) + "\n\n")
		case atom.H4:
			buf.WriteString("#### " + strings.TrimSpace(childMd) + "\n\n")
		case atom.H5:
			buf.WriteString("##### " + strings.TrimSpace(childMd) + "\n\n")
		case atom.H6:
			buf.WriteString("###### " + strings.TrimSpace(childMd) + "\n\n")
		case atom.P:
			// Handle special Vuepress containers like "tip", "warning"
			parentClass := ""
			grandParentClass := ""
			if n.Parent != nil {
				parentClass = getAttr(n.Parent, "class")
				if n.Parent.Parent != nil {
					grandParentClass = getAttr(n.Parent.Parent, "class")
				}
			}
			if strings.Contains(parentClass, "custom-container-title") {
				// Usually inside a div.custom-container.tip/warning etc.
				containerType := "tip" // default
				if strings.Contains(grandParentClass, "warning") {
					containerType = "warning"
				}
				if strings.Contains(grandParentClass, "danger") {
					containerType = "danger"
				}
				// Make it a bold blockquote title
				buf.WriteString("> **" + strings.ToUpper(containerType) + ": " + strings.TrimSpace(childMd) + "**\n")
			} else if strings.Contains(parentClass, "custom-container") && (strings.Contains(parentClass, "tip") || strings.Contains(parentClass, "warning") || strings.Contains(parentClass, "danger")) {
				// Content paragraph within the tip/warning blockquote
				buf.WriteString("> " + strings.TrimSpace(childMd) + "\n")
			} else {
				// Regular paragraph
				buf.WriteString(strings.TrimSpace(childMd) + "\n\n")
			}
		case atom.Ul:
			// Ensure a blank line before list if needed, children LIs handle their lines
			buf.WriteString(childMd + "\n")
		case atom.Ol:
			// Convert ordered list items by iterating and numbering (simplistic numbering)
			itemNum := 1
			var orderedListContent strings.Builder
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if c.Type == html.ElementNode && c.DataAtom == atom.Li {
					orderedListContent.WriteString(fmt.Sprintf("%d. %s", itemNum, strings.TrimSpace(htmlNodeToMarkdown(c))))
					itemNum++
				}
			}
			buf.WriteString(orderedListContent.String() + "\n")
		case atom.Li:
			// Inside UL or OL. UL parent should handle its formatting. OL is handled above.
			// For UL, add the bullet point.
			if n.Parent != nil && n.Parent.DataAtom == atom.Ul {
				buf.WriteString("- " + strings.TrimSpace(childMd) + "\n")
			} else {
				// For OL, just return the content, numbering is done by OL handler
				buf.WriteString(childMd) // Content only
			}
		case atom.A:
			href := getHref(n)
			// Make links absolute if they are relative (common in docs)
			// This part might need adjustment based on the base URL if complex relative paths are used
			if strings.HasPrefix(href, "/") {
				// Attempt to guess base URL - THIS IS A GUESS!
				// A proper solution would require knowing the base URL of the fetched page.
				// For arthas.aliyun.com, it's likely https://arthas.aliyun.com
				href = "https://arthas.aliyun.com" + href
			} else if !strings.HasPrefix(href, "http://") && !strings.HasPrefix(href, "https://") && !strings.HasPrefix(href, "#") {
				// Maybe relative to current path? Difficult to resolve without context. Keep as is for now.
			}
			buf.WriteString("[" + strings.TrimSpace(childMd) + "](" + href + ")")
		case atom.Pre:
			lang := ""
			// Find code block inside pre
			var codeContent string
			var codeNode *html.Node
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if c.Type == html.ElementNode && c.DataAtom == atom.Code {
					codeNode = c
					break
				}
			}
			if codeNode != nil {
				codeContent = getNodeText(codeNode) // Get raw text content of code block
				// Check for language class like "language-java" on the code block
				for _, attr := range codeNode.Attr {
					if attr.Key == "class" {
						if strings.HasPrefix(attr.Val, "language-") {
							lang = strings.TrimPrefix(attr.Val, "language-")
							break
						}
					}
				}
			} else {
				// Fallback: Get text directly from pre if no code tag found
				codeContent = getNodeText(n)
			}
			buf.WriteString("```" + lang + "\n" + strings.TrimSpace(codeContent) + "\n```\n\n")
		case atom.Code:
			// Inline code (if not inside <pre>)
			isBlockCode := false
			if n.Parent != nil && n.Parent.DataAtom == atom.Pre {
				isBlockCode = true
			}
			if !isBlockCode {
				buf.WriteString("`" + strings.TrimSpace(childMd) + "`")
			} // Else: Handled by <pre>
		case atom.Strong, atom.B:
			buf.WriteString("**" + strings.TrimSpace(childMd) + "**")
		case atom.Em, atom.I:
			buf.WriteString("*" + strings.TrimSpace(childMd) + "*")
		case atom.Hr:
			buf.WriteString("---\n\n")
		case atom.Br:
			buf.WriteString("\n") // Simple newline for <br>
		case atom.Table:
			// Basic table handling - assumes simple structure (thead, tbody, tr, th, td)
			buf.WriteString(convertTableToMarkdown(n))
		case atom.Blockquote:
			// Add "> " prefix to each line
			lines := strings.Split(strings.TrimSpace(childMd), "\n")
			for i, line := range lines {
				buf.WriteString("> " + line)
				if i < len(lines)-1 {
					buf.WriteString("\n")
				}
			}
			buf.WriteString("\n\n")

		// Ignore these tags and their content for markdown
		case atom.Script, atom.Style, atom.Button, atom.Svg, atom.Header, atom.Footer, atom.Nav:
			// Ignored tags
			break
		// Default: process children of unknown/unhandled tags
		default:
			buf.WriteString(childMd)
		}

	// Default: return empty for other node types (Comment, Doctype, etc.)
	default:
		return ""
	}

	return buf.String()
}

// Helper to get a specific attribute value
func getAttr(n *html.Node, key string) string {
	for _, attr := range n.Attr {
		if attr.Key == key {
			return attr.Val
		}
	}
	return ""
}

// Basic conversion for simple HTML tables to Markdown tables
func convertTableToMarkdown(tableNode *html.Node) string {
	var md strings.Builder
	var headerRow []string
	var separatorRow []string
	var bodyRows [][]string

	// Find thead and tbody
	var thead, tbody *html.Node
	for c := tableNode.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode {
			if c.DataAtom == atom.Thead {
				thead = c
			} else if c.DataAtom == atom.Tbody {
				tbody = c
			}
		}
	}

	// Process header
	if thead != nil {
		for tr := thead.FirstChild; tr != nil; tr = tr.NextSibling {
			if tr.Type == html.ElementNode && tr.DataAtom == atom.Tr {
				for th := tr.FirstChild; th != nil; th = th.NextSibling {
					if th.Type == html.ElementNode && th.DataAtom == atom.Th {
						headerRow = append(headerRow, strings.TrimSpace(getNodeText(th)))
						separatorRow = append(separatorRow, "---")
					}
				}
				break // Assume only one header row in thead
			}
		}
	}

	// Process body
	if tbody != nil {
		for tr := tbody.FirstChild; tr != nil; tr = tr.NextSibling {
			if tr.Type == html.ElementNode && tr.DataAtom == atom.Tr {
				var currentRow []string
				for td := tr.FirstChild; td != nil; td = td.NextSibling {
					if td.Type == html.ElementNode && (td.DataAtom == atom.Td || td.DataAtom == atom.Th) { // Allow th in body?
						currentRow = append(currentRow, strings.TrimSpace(getNodeText(td)))
					}
				}
				// Ensure row has same number of columns as header (pad if necessary)
				for len(currentRow) < len(headerRow) {
					currentRow = append(currentRow, "")
				}
				if len(currentRow) > 0 {
					bodyRows = append(bodyRows, currentRow[:len(headerRow)]) // Truncate if too long
				}
			}
		}
	}

	// Fallback if no thead found, treat first row of tbody as header
	if len(headerRow) == 0 && len(bodyRows) > 0 {
		headerRow = bodyRows[0]
		bodyRows = bodyRows[1:] // Remove header from body
		for range headerRow {
			separatorRow = append(separatorRow, "---")
		}
	}

	// Assemble Markdown table
	if len(headerRow) > 0 {
		md.WriteString("| " + strings.Join(headerRow, " | ") + " |\n")
		md.WriteString("| " + strings.Join(separatorRow, " | ") + " |\n")
		for _, row := range bodyRows {
			md.WriteString("| " + strings.Join(row, " | ") + " |\n")
		}
		md.WriteString("\n") // Add a blank line after the table
	} else {
		// If table structure is totally unexpected, just return children's text
		md.WriteString(getNodeText(tableNode))
	}

	return md.String()
}
