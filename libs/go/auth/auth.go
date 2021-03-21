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

/*
Package auth helps developers authenticating users to Google Cloud services like Storage, Datastore & Run
*/
package auth

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// authResult can be generated asynchronously from the web server callback or earlier in case of error
type authResult struct {
	token *oauth2.Token
	err   error
}

// RefreshIDToken uses the refresh_token command to get a new id_token
func RefreshIDToken(clientId, clientSecret string, originalToken *oauth2.Token) (string, error) {
	response, err := http.PostForm("https://oauth2.googleapis.com/token",
		url.Values{
			"client_id":     {clientId},
			"client_secret": {clientSecret},
			"grant_type":    {"refresh_token"},
			"refresh_token": {originalToken.RefreshToken},
		})
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	contents, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return "", fmt.Errorf("Failed reading refresh token response: %v", err)
	}
	var parsedResponse map[string]interface{}
	if err := json.Unmarshal(contents, &parsedResponse); err != nil {
		return "", err
	}
	return fmt.Sprintf("%v", parsedResponse["id_token"]), nil
}

// ReadTokenFromFile fetches a saved token from a json file
func ReadTokenFromFile(tokenFilePath string) (*oauth2.Token, error) {
	buffer, err := ioutil.ReadFile(tokenFilePath)
	if err != nil {
		return nil, err
	}
	var token oauth2.Token
	if err := json.Unmarshal(buffer, &token); err != nil {
		return nil, err
	}
	return &token, nil
}

// WriteTokenToFile writes the token in json format to the given path
func WriteTokenToFile(token *oauth2.Token, authJsonFilePath string) error {
	buffer, err := json.Marshal(token)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(authJsonFilePath, buffer, os.ModePerm)
}

// generateRandomId provides a random state for the authentication flow to verify
func generateRandomId() (string, error) {
	buffer := make([]byte, 16)
	_, err := rand.Read(buffer)
	if err != nil {
		return "", fmt.Errorf("error generating random id: %w", err)
	}
	return fmt.Sprintf("%x-%x-%x-%x-%x", buffer[0:4], buffer[4:6], buffer[6:8], buffer[8:10], buffer[10:]), nil
}

// GetTokenInteractively starts a webserver on localhost and a web browser to allow the user to accept the required permissions
func GetTokenInteractively(ctx context.Context, clientID string, clientSecret string, scopes []string) (*oauth2.Token, error) {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return nil, fmt.Errorf("net.Listen failed: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	serverRoot := fmt.Sprintf("http://localhost:%v/", port)
	server := &http.Server{}
	randomState, err := generateRandomId()
	if err != nil {
		return nil, err
	}
	oauthConfig := &oauth2.Config{
		RedirectURL:  serverRoot + "callback",
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Scopes:       scopes,
		Endpoint:     google.Endpoint}
	authResultChannel := make(chan authResult)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		handleGoogleLogin(oauthConfig, randomState, w, r)
	})
	http.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		token, err := handleGoogleCallback(oauthConfig, randomState, w, r)
		go func() {
			//if we close the server too fast it looks like the login failed
			time.Sleep(time.Second)
			server.Shutdown(ctx)
		}()
		authResultChannel <- authResult{token, err}
	})
	go server.Serve(listener)
	fmt.Printf("starting web browser for authentification: %s\n", serverRoot)
	if err := exec.Command("rundll32", "url.dll,FileProtocolHandler", serverRoot).Start(); err != nil {
		return nil, fmt.Errorf("error starting browser: %v", err)
	}
	result := <-authResultChannel
	return result.token, result.err
}

// handleGoogleLogin redirects the web page to the oauth login page
func handleGoogleLogin(oauthConfig *oauth2.Config, oauthState string, w http.ResponseWriter, r *http.Request) {
	url := oauthConfig.AuthCodeURL(oauthState)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

// handleGoogleCallback accepts the result of the login callback
func handleGoogleCallback(oauthConfig *oauth2.Config, oauthState string, w http.ResponseWriter, r *http.Request) (*oauth2.Token, error) {
	if oauthState != r.FormValue("state") {
		fmt.Fprintln(w, "invalid oauth state")
		return nil, fmt.Errorf("invalid oauth state")
	}
	token, err := oauthConfig.Exchange(oauth2.NoContext, r.FormValue("code"))
	if err != nil {
		fmt.Fprintln(w, "code exchange failed: ", err)
		return nil, err
	}
	content, err := fetchUserEmail(token)
	if err != nil {
		fmt.Fprintf(w, "error getting user info: %v", err)
		return nil, err
	}
	var userInfoMap map[string]interface{}
	json.Unmarshal(content, &userInfoMap)
	// confirming to the user which account has been authenticated
	fmt.Fprintf(w, "Thank you %s\n", userInfoMap["email"])
	return token, nil
}

// fetchUserEmail fetches the authenticated user's email
func fetchUserEmail(token *oauth2.Token) ([]byte, error) {
	client := &http.Client{}
	request, err := http.NewRequest("GET", "https://www.googleapis.com/oauth2/v2/userinfo", nil)
	if err != nil {
		return nil, fmt.Errorf("error in http.NewRequest: %v", err)
	}
	token.SetAuthHeader(request)
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("failed getting user info: %s", err.Error())
	}
	defer response.Body.Close()
	contents, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("failed reading response body: %s", err.Error())
	}
	return contents, nil
}
