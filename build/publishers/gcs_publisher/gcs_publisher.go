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

// Binary gcs_publisher will take binaries created by bazel and publish them to a specified location on Google Cloud Storage.
package main

import (
	"bytes"
	"context"
	"crypto/md5"
	"flag"
	"fmt"
	"io"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"strings"
	"time"

	"sge-monorepo/build/cicd/sgeb/buildtool"
	"sge-monorepo/build/cicd/sgeb/protos/buildpb"

	"cloud.google.com/go/storage"
	"github.com/golang/glog"
)

var flags = struct {
	name              string
	bucket            string
	uploadChangedOnly bool
	appendTimestamp   bool
}{}

func main() {
	flag.StringVar(&flags.name, "name", "", "name of the publish unit")
	flag.StringVar(&flags.bucket, "bucket", "", "GCS Bucket to publish to")
	flag.BoolVar(&flags.uploadChangedOnly, "upload_changed_only", false, "whether we only upload changed files")
	flag.BoolVar(&flags.appendTimestamp, "append_timestamp", false, "whether a timestamp should be appended to the file uploaded to the bucket")
	flag.Parse()
	glog.Info("application start")
	glog.Infof("%v", os.Args)

	err := publish()
	if err != nil {
		glog.Errorf("%v", err)
	}
	glog.Info("application exit")
	glog.Flush()

	if err != nil {
		os.Exit(1)
	}
}

func filterFiles(helper buildtool.Helper) []string {
	var artifacts []string
	prefix := "file:///"
	for _, inputs := range helper.Invocation().Inputs {
		for _, f := range inputs.Artifacts {
			// Only publish files
			if f.Uri == "" {
				continue
			}
			if !strings.HasPrefix(f.Uri, prefix) {
				continue
			}
			artifacts = append(artifacts, f.Uri[len(prefix):])
		}
	}
	return artifacts
}

func publish() error {
	gcs, err := storage.NewClient(context.Background())
	if err != nil {
		return fmt.Errorf("cannot create GCS client: %v", err)
	}
	bkt := gcs.Bucket(flags.bucket)
	helper := buildtool.MustLoad()
	artifacts := filterFiles(helper)
	if len(artifacts) != 1 {
		return fmt.Errorf("gcs_publisher only handles single items, have %d items", len(artifacts))
	}
	srcPath := artifacts[0]
	destPath := path.Base(artifacts[0])
	if flags.appendTimestamp {
		t := time.Unix(helper.Invocation().PublishInvocation.InvocationTime.Seconds, 0)
		ext := filepath.Ext(destPath)
		baseName := destPath[0 : len(destPath)-len(ext)]
		destPath = fmt.Sprintf("%s_%s%s", baseName, t.Format("2006_01_02_215004"), ext)
	} else if flags.uploadChangedOnly {
		// Do not bother publishing files that haven't changed.
		// If we are appending a timestamp we can skip this check, file is always neww
		if eq, err := filesEqual(bkt, srcPath, destPath); err != nil {
			return err
		} else if eq {
			fmt.Println("no files changed, nothing to publish")
			helper.MustWritePublishResult(&buildpb.PublishInvocationResult{})
			return nil
		}
	}

	gen, size, err := publishFile(helper, bkt, srcPath, destPath)
	if err != nil {
		return err
	}
	result := &buildpb.PublishResult{
		Name:    flags.name,
		Version: fmt.Sprintf("%d", gen),
		Files: []*buildpb.PublishedFile{
			{
				Size: size,
			},
		},
	}
	helper.MustWritePublishResult(&buildpb.PublishInvocationResult{
		PublishResults: []*buildpb.PublishResult{result},
	})
	return nil
}

func publishFile(helper buildtool.Helper, bkt *storage.BucketHandle, srcPath, destPath string) (int64, int64, error) {
	r, err := os.Open(srcPath)
	if err != nil {
		return 0, 0, err
	}
	defer r.Close()

	obj := bkt.Object(destPath)
	w := obj.NewWriter(context.Background())

	change := helper.Invocation().GetPublishInvocation().GetBaseCl()
	if change != 0 {
		w.Metadata = map[string]string{
			"p4-change": fmt.Sprintf("%d", change),
		}
	} else {
		usr, err := user.Current()
		if err != nil {
			glog.Warningf("can't determine user: %v", err)
			usr = &user.User{Username: "<unknown>"}
		}
		w.Metadata = map[string]string{
			"p4-change": fmt.Sprintf("%s - %v", usr.Username, time.Now()),
		}
	}

	_, err = io.Copy(w, r)
	if err != nil {
		return 0, 0, err
	}
	err = w.Close()
	if err != nil {
		return 0, 0, err
	}
	attrs := w.Attrs()
	return attrs.Generation, attrs.Size, nil
}

func filesEqual(bkt *storage.BucketHandle, src, dest string) (bool, error) {
	attrs, err := bkt.Object(dest).Attrs(context.Background())
	if err != nil {
		if err != storage.ErrObjectNotExist {
			return false, err
		}
		return false, nil
	}
	srcHash, err := hashFile(src)
	if err != nil {
		return false, err
	}
	return bytes.Equal(srcHash, attrs.MD5), nil
}

func hashFile(p string) ([]byte, error) {
	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	hash := md5.New()
	if _, err := io.Copy(hash, f); err != nil {
		return nil, err
	}
	return hash.Sum(nil), nil
}
