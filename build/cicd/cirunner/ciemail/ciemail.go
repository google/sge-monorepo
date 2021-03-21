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

// Package ciemail takes care of the email functionality for CI/CD.
// This main file holds helper function.

package ciemail

import (
	"github.com/julvo/htmlgo"
	"github.com/julvo/htmlgo/attributes"
)

const doctype htmlgo.HTML = `<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.0 Strict//EN" "http://www.w3.org/TR/xhtml1/DTD/xhtml1-strict.dtd">`

// EmailHtml generates an old-school HTML tag.
func EmailHtml(children ...htmlgo.HTML) htmlgo.HTML {
	return doctype + htmlgo.Html(
		htmlgo.Attr(attributes.Attribute{
			Name:  "Xmlns",
			Templ: `{{define "Xmlns"}}xmlns="http://www.w3.org/1999/xhtml"{{end}}`,
		}),
		children...,
	)
}

// CellPadding implements the cellpadding attribute.
func CellPadding(value string) attributes.Attribute {
	return attributes.Attribute{
		Name:  "CellPadding",
		Templ: `{{define "CellPadding"}}cellpadding="` + value + `"{{end}}`,
	}
}

// CellSpacing implements the cellspacing attribute.
func CellSpacing(value string) attributes.Attribute {
	return attributes.Attribute{
		Name:  "CellSpacing",
		Templ: `{{define "CellSpacing"}}cellspacing="` + value + `"{{end}}`,
	}
}

// TableAttr adds some common attributes to tables.
func TableAttr(attrs ...attributes.Attribute) []attributes.Attribute {
	a := []attributes.Attribute{
		CellPadding("0"),
		CellSpacing("0"),
	}
	a = append(a, attrs...)
	return a
}

// TrTd is a simple wrapper for Tr_(Td )).
func TrTd(attrs []attributes.Attribute, children ...htmlgo.HTML) htmlgo.HTML {
	return htmlgo.Tr_(htmlgo.Td(attrs, children...))
}

// TrTd_ is a simple wrapper for Tr_(Td_( )).
func TrTd_(children ...htmlgo.HTML) htmlgo.HTML {
	return htmlgo.Tr_(htmlgo.Td_(children...))
}
