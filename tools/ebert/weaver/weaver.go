// Copyright 2021 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Weaver is a utility to combine html files at build time.  It is intended to
// allow reuse of web components within Ebert.
package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"path/filepath"

	"golang.org/x/net/html"
)

var (
	input   = flag.String("input", "", "Name of input file")
	output  = flag.String("output", "", "Name of output file")
	basedir = flag.String("basedir", "tools/ebert", "Base directory for includes")
)

// weaveScript rewrites script tags with local srcs to embed the script source
// directly.
// For example:
//   <script src="/local_script.js"></script>
// is rewritten as
//   <script>... contents of local_script.js ...</script>
// If the source has already been imported once, the tag is removed.
func weaveScript(node, parent *html.Node, imports map[string]bool) (*html.Node, error) {
	attrs := make([]html.Attribute, 0, len(node.Attr))
	for _, attr := range node.Attr {
		if attr.Key == "src" {
			url, err := url.Parse(attr.Val)
			if err != nil {
				return nil, fmt.Errorf("couldn't parse src URL: %w", err)
			}
			if url.Host == "" {
				// Treat as an import.
				// First check if it's already imported.
				if imports[url.Path] {
					if node.Parent == parent && parent != nil {
						parent.RemoveChild(node)
					}
					return nil, nil
				}

				path := filepath.Join(*basedir, url.Path)
				in, err := os.Open(path)
				if err != nil {
					return nil, fmt.Errorf("couldn't open script %s: %w", path, err)
				}
				data, err := ioutil.ReadAll(in)
				if err != nil {
					return nil, fmt.Errorf("couldn't read script %s: %w", path, err)
				}
				node.AppendChild(&html.Node{
					Type: html.TextNode,
					Data: string(data),
				})
				imports[url.Path] = true
				continue
			}
		}
		attrs = append(attrs, attr)
	}
	node.Attr = attrs
	return node, nil
}

// weaveComponent replaces <use-component> tags with the component
// implementation.
// For example:
//   <use-component src="/local_component.html"></use-component>
// is rewritten as
//   ... contents of local_component.html ...
// If a component has already been imported, the <use-component> tag is
// simply removed.
func weaveComponent(node, parent, loc *html.Node, imports map[string]bool) error {
	defer func(node, parent *html.Node) {
		if node.Parent == parent && parent != nil {
			parent.RemoveChild(node)
		}
	}(node, parent)

	for _, attr := range node.Attr {
		if attr.Key == "src" {
			url, err := url.Parse(attr.Val)
			if err != nil {
				return fmt.Errorf("couldn't parse src URL: %w", err)
			}
			// Check if the component has already been imported.
			if imports[url.Path] {
				return nil
			}
			path := filepath.Join(*basedir, url.Path)
			in, err := os.Open(path)
			if err != nil {
				return fmt.Errorf("couldn't open component %s: %w", path, err)
			}
			fragments, err := html.ParseFragment(in, parent)
			if err != nil {
				return fmt.Errorf("couldn't parse component %s: %w", path, err)
			}
			for _, fragment := range fragments {
				fragment, err = weaveNode(fragment, parent, loc, imports)
				if fragment != nil && parent != nil {
					parent.InsertBefore(fragment, loc)
				}
			}
			imports[url.Path] = true
		}
	}
	return nil
}

// weaveNode rewrites an HTML tree, rewriting <script> and <use-component> tags
// with the referenced contents.  In the case of <use-component>, the rewriting
// is recursive (so if the component itself contains <use-component> tags, they
// will also be rewritten).
func weaveNode(node, parent, loc *html.Node, imports map[string]bool) (*html.Node, error) {
	if node.Type == html.ElementNode && node.Data == "script" {
		return weaveScript(node, parent, imports)
	}
	if node.Type == html.ElementNode && node.Data == "use-component" {
		return nil, weaveComponent(node, parent, loc, imports)
	}

	child := node.FirstChild
	for child != nil {
		next := child.NextSibling
		if _, err := weaveNode(child, node, child, imports); err != nil {
			return nil, err
		}
		child = next
	}
	return node, nil
}

func main() {
	flag.Parse()

	in, err := os.Open(*input)
	if err != nil {
		log.Fatalf("error opening %s: %v", *input, err)
	}

	out, err := os.Create(*output)
	if err != nil {
		log.Fatalf("error creating %s: %v", *output, err)
	}

	doc, err := html.Parse(in)
	if err != nil {
		log.Fatalf("error parsing %s: %v", *input, err)
	}

	imports := map[string]bool{}
	if _, err = weaveNode(doc, nil, nil, imports); err != nil {
		log.Fatalf("error weaving %s: %v", *input, err)
	}

	if err = html.Render(out, doc); err != nil {
		log.Fatalf("error writing %s: %v", *output, err)
	}
}
