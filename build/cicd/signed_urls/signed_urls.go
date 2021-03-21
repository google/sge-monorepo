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

import(
    "flag"
    "fmt"
    "time"
    "context"
    "os"
    "os/exec"
    "strings"

    "cloud.google.com/go/iam/credentials/apiv1"
    "cloud.google.com/go/storage"
    credentialspb "google.golang.org/genproto/googleapis/iam/credentials/v1"
)

// getDefaultServiceAccount is a hack that uses gcloud to obtain the default service account user.
func getDefaultServiceAccount() (string, error) {
    args := []string{"gcloud", "config", "get-value", "core/account"}
    out, err := exec.Command(args[0], args[1:]...).CombinedOutput()
    if err != nil {
        return "", fmt.Errorf("could not run %s: %w", args, err)
    }
    return strings.Trim(string(out), " \r\n"), err
}

func internalMain() error {
    f := struct {
        bucket string
        object string
        serviceAccount string
    }{}
    flag.StringVar(&f.bucket, "bucket", "", "GCS bucket name")
    flag.StringVar(&f.object, "object", "", "Object within the bucket")
    flag.StringVar(&f.serviceAccount, "service-account", "", "Service account to use")
    flag.Parse()

    serviceAccount := f.serviceAccount
    if serviceAccount == "" {
        sa, err := getDefaultServiceAccount()
        if err != nil {
            return fmt.Errorf("could not get default service account: %w", err)
        }
        serviceAccount = sa
    }
    fmt.Println("SERVICE ACCOUNT:", serviceAccount)

    ctx := context.Background()
    credsClient, err := credentials.NewIamCredentialsClient(ctx)
    if err != nil {
        return fmt.Errorf("could not get IAM creds client: %w", err)
    }
    url, err := storage.SignedURL(f.bucket, f.object, &storage.SignedURLOptions{
        Method: "GET",
        GoogleAccessID: serviceAccount,
        Expires: time.Now().Add(4 * time.Hour),
        SignBytes: func(bytes []byte) ([]byte, error) {
            resp, err := credsClient.SignBlob(ctx, &credentialspb.SignBlobRequest{
                Payload: bytes,
                Name: serviceAccount,
            })
            if err != nil {
                return nil, fmt.Errorf("could not sign blob: %w", err)
            }
            return resp.SignedBlob, nil
        },
    })
    if err != nil {
        return fmt.Errorf("could not create signed URL: %w", err)
    }
    fmt.Println("URL:", url)
    return nil
}

func main() {
    // if err := internalMain(); err != nil {
    //     fmt.Printf("ERROR: %v\n", err)
    //     os.Exit(1)
    // }
    out, err := exec.Command("echo", "hello").CombinedOutput()
    if err != nil {
        fmt.Printf("ERROR: %v\n", err)
        os.Exit(1)
    }
    for i := 0; i < 120; i++ {
        fmt.Println("ECHO:", i, string(out))
        time.Sleep(1 * time.Second)
    }
}
