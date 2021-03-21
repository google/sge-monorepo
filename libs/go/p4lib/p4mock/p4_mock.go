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

// Mock implements a lightweight mock for the P4 interface.
//
// Usage:
//      p4 := p4mock.New()
//      p4.ClientResponses["my-client"] = ...
//      ...
package p4mock

import (
	"fmt"
	"time"

	"sge-monorepo/libs/go/p4lib"
)

// Mock is meant to provide a lightweight mechanism to provide your own callbacks into p4lib.
// Usage:
//
//      p4 := p4mock.New()
//      p4.ClientFunc = func(clientName string) (*p4lib.Client, error) {
//          if client, ok := someMap[clientName]; ok {
//              return client, nil
//          }
//          return nil, fmt.Errorf("client % not expected", clientName)
//      })
//
//      ...
//
//      err := SomeCallThatRequiresPerforce(p4, args...)
//
type Mock struct {
	AddFunc                func(paths []string, options ...string) (string, error)
	AddDirFunc             func(dir string, options ...string) (string, error)
	ChangeFunc             func(desc string) (int, error)
	ChangeUpdateFunc       func(desc string, cl int) error
	ChangesFunc            func(args ...string) ([]p4lib.Change, error)
	ClientFunc             func(clientName string) (*p4lib.Client, error)
	ClientSetFunc          func(client *p4lib.Client) (string, error)
	ClientsFunc            func() ([]string, error)
	DeleteFunc             func(paths []string, cl int) (string, error)
	DescribeFunc           func(cl []int) ([]p4lib.Description, error)
	DescribeShelvedFunc    func(cls ...int) ([]p4lib.Description, error)
	DiffFileFunc           func(file string) error
	DiffFunc               func(file0 string, file1 string) ([]p4lib.Diff, error)
	Diff2Func              func(file0 string, file1 string) ([]p4lib.Diff, error)
	DirsFunc               func(root string) ([]string, error)
	EditFunc               func(paths []string, cl int) (string, error)
	ExecCmdFunc            func(args ...string) (string, error)
	ExecCmdWithOptionsFunc func(args []string, opts ...p4lib.Option) (string, error)
	FilesFunc              func(files ...string) ([]p4lib.FileDetails, error)
	FstatFunc              func(args ...string) (*p4lib.FstatResult, error)
	GrepFunc               func(pattern string, caseSensitive bool, depotPaths ...string) ([]p4lib.Grep, error)
	GrepLargeFunc          func(pattern string, depotPath string, caseSensitive bool, status *p4lib.GrepStatus) error
	HaveFunc               func(patterns ...string) ([]p4lib.File, error)
	IndexFunc              func(name string, attr int, values ...string) error
	IndexDeleteFunc        func(name string, attr int, values ...string) error
	InfoFunc               func() (*p4lib.Info, error)
	IgnoresFunc            func(paths []string) (string, error)
	KeyGetFunc             func(key string) (string, error)
	KeySetFunc             func(key, val string) error
	KeyIncFunc             func(key string) (string, error)
	KeyCasFunc             func(key, oldval, newval string) error
	KeysFunc               func(pattern string) (map[string]string, error)
	LoginFunc              func(user string) (string, time.Time, error)
	OpenedFunc             func(change string) ([]p4lib.OpenedFile, error)
	PrintFunc              func(args ...string) (string, error)
	PrintExFunc            func(files ...string) ([]p4lib.FileDetails, error)
	ReconcileFunc          func(paths []string, cl int) (string, error)
	RevertFunc             func(paths []string, opts ...string) (string, error)
	SetFunc                func(key, value string) error
	SizesFunc              func(dirs ...string) (*p4lib.SizeCollection, error)
	SubmitFunc             func(cl int, options ...string) (string, error)
	SyncFunc               func(targets []string, options ...string) (string, error)
	SyncSizeFunc           func(targets []string) (*p4lib.SyncSize, error)
	TicketsFunc            func(args ...string) ([]p4lib.Ticket, error)
	TrustFunc              func(args ...string) error
	UnshelveFunc           func(cl int, args ...string) (string, error)
	UsersFunc              func() ([]p4lib.User, error)
	VerifiedUnshelveFunc   func(cl int) (string, error)
	WhereFunc              func(path string) (string, error)
	WhereExFunc            func(paths []string) ([]string, error)
	MoveFunc               func(cl int, from string, to string) (string, error)
}

func New() Mock {
	return Mock{}
}

func (p4 Mock) Add(paths []string, options ...string) (string, error) {
	if p4.AddFunc == nil {
		return "", fmt.Errorf("AddFunc not set")
	}
	return p4.AddFunc(paths, options...)
}

func (p4 Mock) AddDir(dir string, options ...string) (string, error) {
	if p4.AddDirFunc == nil {
		return "", fmt.Errorf("AddDir not set")
	}
	return p4.AddDirFunc(dir, options...)
}

func (p4 Mock) Change(desc string) (int, error) {
	if p4.ChangeFunc == nil {
		return 0, fmt.Errorf("Change not set")
	}
	return p4.ChangeFunc(desc)
}

func (p4 Mock) ChangeUpdate(desc string, cl int) error {
	if p4.ChangeUpdateFunc == nil {
		return fmt.Errorf("ChangeUpdateFunc not set")
	}
	return p4.ChangeUpdateFunc(desc, cl)
}

func (p4 Mock) Changes(args ...string) ([]p4lib.Change, error) {
	if p4.ChangesFunc == nil {
		return nil, fmt.Errorf("Changes not set")
	}
	return p4.ChangesFunc(args...)
}

func (p4 Mock) Client(clientName string) (*p4lib.Client, error) {
	if p4.ClientFunc == nil {
		return nil, fmt.Errorf("Client not set")
	}
	return p4.ClientFunc(clientName)
}

func (p4 Mock) ClientSet(client *p4lib.Client) (string, error) {
	if p4.ClientSetFunc == nil {
		return "", fmt.Errorf("ClientSetFunc not set")
	}
	return p4.ClientSetFunc(client)
}

func (p4 Mock) Clients() ([]string, error) {
	if p4.ClientsFunc == nil {
		return nil, fmt.Errorf("ClientsFunc not set")
	}
	return p4.ClientsFunc()
}

func (p4 Mock) Delete(paths []string, cl int) (string, error) {
	if p4.DeleteFunc == nil {
		return "", fmt.Errorf("DeleteFunc not set")
	}
	return p4.DeleteFunc(paths, cl)
}

func (p4 Mock) Describe(cls []int) ([]p4lib.Description, error) {
	if p4.DescribeFunc == nil {
		return nil, fmt.Errorf("DescribeFunc not set")
	}
	return p4.DescribeFunc(cls)
}

func (p4 Mock) DescribeShelved(cls ...int) ([]p4lib.Description, error) {
	if p4.DescribeShelvedFunc == nil {
		return nil, fmt.Errorf("DescribeShelvedFunc not set")
	}
	return p4.DescribeShelvedFunc(cls...)
}

func (p4 Mock) DiffFile(file string) error {
	if p4.DiffFileFunc == nil {
		return fmt.Errorf("DiffFileFunc not set")
	}
	return p4.DiffFileFunc(file)
}

func (p4 Mock) Diff(file0 string, file1 string) ([]p4lib.Diff, error) {
	if p4.DiffFunc == nil {
		return nil, fmt.Errorf("DiffFunc not set")
	}
	return p4.DiffFunc(file0, file1)

}

func (p4 Mock) Diff2(file0 string, file1 string) ([]p4lib.Diff, error) {
	if p4.Diff2Func == nil {
		return nil, fmt.Errorf("Diff2Func not set")
	}
	return p4.Diff2Func(file0, file1)
}

func (p4 Mock) Dirs(root string) ([]string, error) {
	if p4.DirsFunc == nil {
		return nil, fmt.Errorf("DirsFunc not set")
	}
	return p4.DirsFunc(root)
}

func (p4 Mock) Edit(paths []string, cl int) (string, error) {
	if p4.EditFunc == nil {
		return "", fmt.Errorf("EditFunc not set")
	}
	return p4.EditFunc(paths, cl)
}

func (p4 Mock) ExecCmd(args ...string) (string, error) {
	if p4.ExecCmdFunc == nil {
		return "", fmt.Errorf("ExecCmdFunc not set")
	}
	return p4.ExecCmdFunc(args...)
}

func (p4 Mock) ExecCmdWithOptions(args []string, opts ...p4lib.Option) (string, error) {
	if p4.ExecCmdWithOptionsFunc == nil {
		return "", fmt.Errorf("ExecCmdWithOptionsFunc not set")
	}
	return p4.ExecCmdWithOptionsFunc(args, opts...)
}

func (p4 Mock) Files(files ...string) ([]p4lib.FileDetails, error) {
	if p4.FilesFunc == nil {
		return nil, fmt.Errorf("FilesFunc not set")
	}
	return p4.FilesFunc(files...)
}

func (p4 Mock) Fstat(args ...string) (*p4lib.FstatResult, error) {
	if p4.FstatFunc == nil {
		return nil, fmt.Errorf("FstatFunc not set")
	}
	return p4.FstatFunc(args...)
}

func (p4 Mock) Grep(pattern string, caseSensitive bool, depotPaths ...string) ([]p4lib.Grep, error) {
	if p4.GrepFunc == nil {
		return nil, fmt.Errorf("GrepFunc not set")
	}
	return p4.GrepFunc(pattern, caseSensitive, depotPaths...)
}

func (p4 Mock) GrepLarge(pattern string, depotPath string, caseSensitive bool, status *p4lib.GrepStatus) error {
	if p4.GrepLargeFunc == nil {
		return fmt.Errorf("GrepLargeFunc not set")
	}
	return p4.GrepLargeFunc(pattern, depotPath, caseSensitive, status)
}

func (p4 Mock) Have(patterns ...string) ([]p4lib.File, error) {
	if p4.HaveFunc == nil {
		return nil, fmt.Errorf("HaveFunc not set")
	}
	return p4.HaveFunc(patterns...)
}

func (p4 Mock) Index(name string, attr int, values ...string) error {
	if p4.IndexFunc == nil {
		return fmt.Errorf("IndexFunc not set")
	}
	return p4.IndexFunc(name, attr, values...)
}

func (p4 Mock) IndexDelete(name string, attr int, values ...string) error {
	if p4.IndexDeleteFunc == nil {
		return fmt.Errorf("IndexDeleteFunc not set")
	}
	return p4.IndexDeleteFunc(name, attr, values...)
}

func (p4 Mock) Info() (*p4lib.Info, error) {
	if p4.InfoFunc == nil {
		return nil, fmt.Errorf("InfoFunc not set")
	}
	return p4.InfoFunc()
}

func (p4 Mock) Ignores(paths []string) (string, error) {
	if p4.IgnoresFunc == nil {
		return "", fmt.Errorf("IgnoresFunc not set")
	}
	return p4.IgnoresFunc(paths)
}

func (p4 Mock) KeyGet(key string) (string, error) {
	if p4.KeyGetFunc == nil {
		return "", fmt.Errorf("KeyGetFunc not set")
	}
	return p4.KeyGetFunc(key)
}

func (p4 Mock) KeySet(key, val string) error {
	if p4.KeySetFunc == nil {
		return fmt.Errorf("KeySetFunc not set")
	}
	return p4.KeySetFunc(key, val)
}

func (p4 Mock) KeyInc(key string) (string, error) {
	if p4.KeyIncFunc == nil {
		return "", fmt.Errorf("KeyIncFunc not set")
	}
	return p4.KeyIncFunc(key)
}

func (p4 Mock) KeyCas(key, oldval, newval string) error {
	if p4.KeyCasFunc == nil {
		return fmt.Errorf("KeyCasFunc not set")
	}
	return p4.KeyCasFunc(key, oldval, newval)
}

func (p4 Mock) Keys(pattern string) (map[string]string, error) {
	if p4.KeysFunc == nil {
		return nil, fmt.Errorf("KeysFunc not set")
	}
	return p4.KeysFunc(pattern)
}

func (p4 Mock) Login(user string) (string, time.Time, error) {
	if p4.LoginFunc == nil {
		return "", time.Time{}, fmt.Errorf("LoginFunc not set")
	}
	return p4.LoginFunc(user)
}

func (p4 Mock) Opened(change string) ([]p4lib.OpenedFile, error) {
	if p4.OpenedFunc == nil {
		return nil, fmt.Errorf("OpenedFunc not set")
	}
	return p4.OpenedFunc(change)
}

func (p4 Mock) Print(args ...string) (string, error) {
	if p4.PrintFunc == nil {
		return "", fmt.Errorf("PrintFunc not set")
	}
	return p4.PrintFunc(args...)
}

func (p4 Mock) PrintEx(files ...string) ([]p4lib.FileDetails, error) {
	if p4.PrintExFunc == nil {
		return nil, fmt.Errorf("PrintExFunc not set")
	}
	return p4.PrintExFunc(files...)
}

func (p4 Mock) Reconcile(paths []string, cl int) (string, error) {
	if p4.ReconcileFunc == nil {
		return "", fmt.Errorf("ReconcileFunc not set")
	}
	return p4.ReconcileFunc(paths, cl)
}

func (p4 Mock) Revert(paths []string, opts ...string) (string, error) {
	if p4.RevertFunc == nil {
		return "", fmt.Errorf("RevertFunc not set")
	}
	return p4.RevertFunc(paths, opts...)
}

func (p4 Mock) Set(key, value string) error {
	if p4.SetFunc == nil {
		return fmt.Errorf("SetFunc not set")
	}
	return p4.SetFunc(key, value)
}

func (p4 Mock) Sizes(dirs ...string) (*p4lib.SizeCollection, error) {
	if p4.SizesFunc == nil {
		return nil, fmt.Errorf("SizesFunc not set")
	}
	return p4.SizesFunc(dirs...)
}

func (p4 Mock) Submit(cl int, options ...string) (string, error) {
	if p4.SubmitFunc == nil {
		return "", fmt.Errorf("SubmitFunc not set")
	}
	return p4.SubmitFunc(cl, options...)
}

func (p4 Mock) Sync(targets []string, options ...string) (string, error) {
	if p4.SyncFunc == nil {
		return "", fmt.Errorf("SyncFunc not set")
	}
	return p4.SyncFunc(targets, options...)
}

func (p4 Mock) SyncSize(targets []string) (*p4lib.SyncSize, error) {
	if p4.SyncSizeFunc == nil {
		return nil, fmt.Errorf("SyncSizeFunc not set")
	}
	return p4.SyncSizeFunc(targets)
}

func (p4 Mock) Tickets(args ...string) ([]p4lib.Ticket, error) {
	if p4.TicketsFunc == nil {
		return nil, fmt.Errorf("TicketsFunc not set")
	}
	return p4.TicketsFunc(args...)
}

func (p4 Mock) Trust(args ...string) error {
	if p4.TrustFunc == nil {
		return fmt.Errorf("TrustFunc not set")
	}
	return p4.TrustFunc(args...)
}

func (p4 Mock) Unshelve(cl int, args ...string) (string, error) {
	if p4.UnshelveFunc == nil {
		return "", fmt.Errorf("UnshelveFunc not set")
	}
	return p4.UnshelveFunc(cl, args...)
}

func (p4 Mock) Users() ([]p4lib.User, error) {
	if p4.UsersFunc == nil {
		return nil, fmt.Errorf("UsersFunc not set")
	}
	return p4.UsersFunc()
}

func (p4 Mock) VerifiedUnshelve(cl int) (string, error) {
	if p4.VerifiedUnshelveFunc == nil {
		return "", fmt.Errorf("VerifiedUnshelveFunc not set")
	}
	return p4.VerifiedUnshelveFunc(cl)
}

func (p4 Mock) Where(path string) (string, error) {
	if p4.WhereFunc == nil {
		return "", fmt.Errorf("WhereFunc not set")
	}
	return p4.WhereFunc(path)
}

func (p4 Mock) WhereEx(paths []string) ([]string, error) {
	if p4.WhereExFunc == nil {
		return nil, fmt.Errorf("WhereExFunc not set")
	}
	return p4.WhereExFunc(paths)
}

func (p4 Mock) Move(cl int, from string, to string) (string, error) {
	if p4.MoveFunc == nil {
		return "", fmt.Errorf("MoveFunc not set")
	}
	return p4.MoveFunc(cl, from, to)
}
