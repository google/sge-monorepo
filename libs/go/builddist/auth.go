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

package builddist

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"

	"sge-monorepo/libs/go/auth"

	"golang.org/x/oauth2"
	"google.golang.org/api/option"
)

// AuthRenewSource is an implementation of the oauth2.TokenSource interface to refresh the original token
//    to be used in conjunction with oauth2.ReuseTokenSource
type AuthRenewSource struct {
	OriginalToken *oauth2.Token
	ClientID      string
	ClientSecret  string
}

// AuthRenewSource.Token refreshes the old refresh_token and returns a new access token
func (self AuthRenewSource) Token() (*oauth2.Token, error) {
	return refreshToken(self.ClientID, self.ClientSecret, self.OriginalToken)
}
func refreshToken(clientId string, clientSecret string, token *oauth2.Token) (*oauth2.Token, error) {
	response, err := http.PostForm("https://oauth2.googleapis.com/token",
		url.Values{
			"client_id":     {clientId},
			"client_secret": {clientSecret},
			"grant_type":    {"refresh_token"},
			"refresh_token": {token.RefreshToken},
		})
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	contents, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("Failed reading refresh token response: %v", err)
	}
	var refreshedToken oauth2.Token
	if err := json.Unmarshal(contents, &refreshedToken); err != nil {
		return nil, fmt.Errorf("Failed to parse refreshed token: %v", err)
	}
	return &refreshedToken, nil
}

// MakeDefaultAuthClientOption returns the necessary option to authenticate the user
func MakeDefaultAuthClientOption(ctx context.Context, config PackageConfig) option.ClientOption {
	tokenSource, err := MakeTokenSourceFromConfig(ctx, config.Auth)
	if err != nil {
		log.Fatal("Authentication error: ", err)
	}
	return option.WithTokenSource(tokenSource)
}

// MakeTokenSourceFromConfig implements the authentication flow as configured in the json config file
func MakeTokenSourceFromConfig(ctx context.Context, config AuthConfig) (oauth2.TokenSource, error) {
	cachedToken, err := auth.ReadTokenFromFile(config.TokenCacheFilename)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("Cached token not found, requesting new authentification.\n")
		} else {
			log.Printf("No valid cached token: %v\n Requesting new authentication.\n", err)
		}
	}
	return makeTokenSource(ctx, cachedToken, config.TokenCacheFilename, config.ClientID, config.ClientSecret, config.Scopes)
}

// makeTokenSource returns a oauth2.TokenSource to be provided to the client APIs in a blocking manner.
//    Otherwise, if there is cached token in [authJsonFilePath] it will to use it
//       - directly if it's still valid
//       - by renewing it if possible
//    Otherwise, it will ask the user by starting a web server bound to localhost and starting the browser. The resulting token will be saved in [authJsonFilePath].
//    This call will block.
func makeTokenSource(
	ctx context.Context,
	cachedToken *oauth2.Token,
	authJsonFilePath string,
	clientID string,
	clientSecret string,
	scopes []string) (oauth2.TokenSource, error) {
	if cachedToken == nil {
		var err error
		cachedToken, err = auth.GetTokenInteractively(ctx, clientID, clientSecret, scopes)
		if err != nil {
			return nil, err
		}
		//this original token constains a refresh_token field that allows us to get more access tokens
		if err := auth.WriteTokenToFile(cachedToken, authJsonFilePath); err != nil {
			fmt.Println("Error saving token file: ", err)
		}
	}
	renewSource := AuthRenewSource{
		OriginalToken: cachedToken,
		ClientID:      clientID,
		ClientSecret:  clientSecret}
	accessToken := cachedToken
	if !(*accessToken).Valid() {
		newToken, err := renewSource.Token()
		if !newToken.Valid() || err != nil {
			fmt.Println("Error renewing token ", err) //can happen if cached token is no longer valid
			return makeTokenSource(ctx, nil, authJsonFilePath, clientID, clientSecret, scopes)
		}
		accessToken = newToken
		//we don't save renewed tokens - they don't contain a refresh_token field and are therefore dead ends
	}
	//will keep accessToken for as long as it's valid
	return oauth2.ReuseTokenSource(accessToken, renewSource), nil
}
