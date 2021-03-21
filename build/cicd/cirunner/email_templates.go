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

package main

import (
	"html/template"
)

type templateData struct {
	Author     string
	ReviewID   int
	ReviewURL  string
	EbertURL   string
	ResultsURL string
}

var startTemplate = template.Must(template.New("startTemplate").Parse(`
<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.0 Strict//EN" "http://www.w3.org/TR/xhtml1/DTD/xhtml1-strict.dtd">
<html xmlns="http://www.w3.org/1999/xhtml">
  <head>
    <meta http-equiv="Content-Type" content="text/html; charset=UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0"/>
    <link href="https://fonts.googleapis.com/css2?family=Roboto&display=swap" rel="stylesheet">
		<style>
			.banner {
                background-color: #3498db;
				font-size: 30px;
				color: white;
			}
			.btn a {
				background-color: #ffffff;
				border: solid 1px #3498db;
				border-radius: 5px;
				box-sizing: border-box;
				color: #3498db;
				cursor: pointer;
				display: inline-block;
				font-size: 14px;
				font-weight: bold;
				margin: 0;
				padding: 12px 25px;
				text-decoration: none;
				text-transform: capitalize;
      }
      .btn-primary table td {
        background-color: #3498db;
      }
      .btn-primary a {
        background-color: #3498db;
        border-color: #3498db;
        color: #ffffff;
      }
    </style>
  </head>
  <body style="margin: 0; padding: 0; font-family:'Roboto', sans-serif;">
    <table border="0" cellpadding="0" cellspacing="0" width="100%">
      <tr><td>
        <table align="center" border="0" cellpadding="0" cellspacing="0" width="600" style="border-collapse: collapse;">
          <tr class="banner">
            <td align="center" style="padding: 40px 0 20px 0;">
            <img src="https://fonts.gstatic.com/s/i/googlematerialicons/cached/v6/white-36dp/2x/gm_cached_white_36dp.png" alt="Success" style="display: block;"/>
            </td>
          </tr>
          <tr class="banner">
            <td align="center" style="padding-bottom: 20px;">Running presubmit for Review {{.ReviewID}}</td>
          </tr>
		  <tr>
			<td style="padding:10px;">
			  <p>Hello {{.Author}},</p>
			  <p>CI is running a presubmit on your CL.</p>
			</td>
          </tr>
          <tr>
            <td>
              <table width="100%" border="0" cellpadding="0">
                <tr>
                  <td align="center" width="34%" class="btn btn-primary">
                    <a target="_blank" href="{{.ReviewURL}}">See Review</a>
                  </td>
                  <td align="center" width="33%" class="btn btn-primary">
                    <a target="_blank" href="{{.EbertURL}}">Ebert Review</a>
                  </td>
                  <td align="center" width="33%" class="btn btn-primary">
                    <a target="_blank" href="{{.ResultsURL}}">See Logs</a>
                  </td>
                </tr>
              </table>
            </td>
          </tr>
        </table>
      </td></tr>
    </table>
  </body>
</html>
`))

var failTemplate = template.Must(template.New("failTemplate").Parse(`
<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.0 Strict//EN" "http://www.w3.org/TR/xhtml1/DTD/xhtml1-strict.dtd">
<html xmlns="http://www.w3.org/1999/xhtml">
  <head>
    <meta http-equiv="Content-Type" content="text/html; charset=UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0"/>
    <link href="https://fonts.googleapis.com/css2?family=Roboto&display=swap" rel="stylesheet">
		<style>
			.banner {
				background-color: #ee4c40;
				font-size: 30px;
				color: white;
			}
			.btn a {
				background-color: #ffffff;
				border: solid 1px #3498db;
				border-radius: 5px;
				box-sizing: border-box;
				color: #3498db;
				cursor: pointer;
				display: inline-block;
				font-size: 14px;
				font-weight: bold;
				margin: 0;
				padding: 12px 25px;
				text-decoration: none;
				text-transform: capitalize;
      }
      .btn-primary table td {
        background-color: #3498db;
      }
      .btn-primary a {
        background-color: #3498db;
        border-color: #3498db;
        color: #ffffff;
      }
    </style>
  </head>
  <body style="margin: 0; padding: 0; font-family:'Roboto', sans-serif;">
    <table border="0" cellpadding="0" cellspacing="0" width="100%">
      <tr><td>
        <table align="center" border="0" cellpadding="0" cellspacing="0" width="600" style="border-collapse: collapse;">
          <tr class="banner">
            <td align="center" style="padding: 40px 0 20px 0;">
              <img src="https://fonts.gstatic.com/s/i/googlematerialicons/error/v8/white-36dp/2x/gm_error_white_36dp.png" alt="Success" style="display: block;"/>
            </td>
          </tr>
          <tr class="banner">
            <td align="center" style="padding-bottom: 20px;">Presubmit failure for Review {{.ReviewID}}</td>
          </tr>
		  <tr>
			<td style="padding:10px;">
			  <p>Hello {{.Author}},</p>
			  <p>CI run detected a CI build failure on your CL.</p>
			  <p>This means that the presubmit machinery was not able to run.</p>
			  <p>If you're not working on toolchains, build systems or CI, it is very possible that this failure is not your fault.</p>
              <p>Please refer to the <a target="_blank" href="https://dynamite-preprod.sandbox.google.com/room/AAAAc9NqxPk">SG&E CI/CD chatroom</a> for assistance</p>
			</td>
          </tr>
          <tr>
            <td>
              <table width="100%" border="0" cellpadding="0">
                <tr>
                  <td align="center" width="34%" class="btn btn-primary">
                    <a target="_blank" href="{{.ReviewURL}}">See Review</a>
                  </td>
                  <td align="center" width="33%" class="btn btn-primary">
                    <a target="_blank" href="{{.EbertURL}}">Ebert</a>
                  </td>
                  <td align="center" width="33%" class="btn btn-primary">
                    <a target="_blank" href="{{.ResultsURL}}">See Logs</a>
                  </td>
                </tr>
              </table>
            </td>
          </tr>
        </table>
      </td></tr>
    </table>
  </body>
</html>
`))
