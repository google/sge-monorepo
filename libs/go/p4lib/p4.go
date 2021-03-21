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

// package p4lib wraps perforce CLI commands in a convenient interface
package p4lib

import (
	"fmt"
	"io"
	"strings"
	"time"
)

var (
	ErrFileNotFound = fmt.Errorf("no matching files")
	ErrKeyNotFound  = fmt.Errorf("p4 key not found")
	ErrCasMismatch  = fmt.Errorf("check-and-set mismatch, new value not set")
)

// P4 is an abstract interface you can use to call into Perforce.
// That way users can use this interface and use P4Mock (or some other implementation of their own)
// to test their code against a fake Perforce.
//
// Usage:
//      p4 := p4lib.New()
//      client, err := p4.Client("my-client")
//      ...
//
// NOTE: There are also options available that can be provided at creation time.
// Example:
//
//      p4 := p4lib.New(OutputOption(os.Stdout))
//
// The list of options is defined after the |P4| interface.
type P4 interface {
	// Add executes a p4 add, marking everything in paths for add using the options received as params
	Add(paths []string, options ...string) (string, error)

	// AddDir executes a p4 add for everything in directory dir and adds it using the options received as params
	AddDir(dir string, options ...string) (string, error)

	// Change executes a p4 change command and creates a new changelist with specified description.
	Change(desc string) (int, error)

	// ChangeUpdate executes a p4 change command to update specified CL with new description
	ChangeUpdate(desc string, cl int) error

	// Changes executes a p4 changes command and returns a slice of p4 change details.
	Changes(args ...string) ([]Change, error)

	// If clientName is empty, it returns the default P4CLIENT.
	Client(clientName string) (*Client, error)

	// ClientSet commits the given client configuration into the server.
	// Whether there is an error or not, the command returns stdout/stderr.
	ClientSet(client *Client) (string, error)

	// Clients returns all the client names currently present with the server.
	// The returned list will be sorted.
	Clients() ([]string, error)

	// Delete executes a p4 delete, marking everything in paths for deletion in changelist cl.
	// 0 means the default changelist.
	Delete(paths []string, cl int) (string, error)

	// Describes invokes a "p4 describe" that gives details about a changelist.
	Describe(cl []int) ([]Description, error)

	// DescribeShelved runs a "p4 describe" but returns the shelved files within a CL.
	DescribeShelved(cls ...int) ([]Description, error)

	// Diff opens the P4Merge to diff between a local file and its revisions on the perforce server.
	DiffFile(file string) error

	// Diff executes a "p4 diff" command that performs a diff between a local file and file that
	// exists on the perforce server.
	Diff(file0 string, file1 string) ([]Diff, error)

	// Diff2 executes a "p4 diff2" command that performs a diff between two files that exist
	// remotely on the perforce server.
	Diff2(file0 string, file1 string) ([]Diff, error)

	// Dirs invokes "p4 dirs" and returns a list of subdirectories in specific root folder.
	Dirs(root string) ([]string, error)

	// Edit executes a p4 edit of every file in the paths slice and adds them to changelist cl.
	Edit(paths []string, cl int) (string, error)

	// ExecCmd executes a perforce command with specified arguments.
	// Returns the command output and any possible errors.
	// If Stdout or Stderr is overriden in the implementation, the output will be diverted that
	// way and won't be returned as a value.
	ExecCmd(args ...string) (string, error)

	// ExecCmdWithOptions permits to run a p4 command with some changes to the underlying
	// functionality. This is meant for advanced usage.
	ExecCmdWithOptions(args []string, opts ...Option) (string, error)

	// Files invokes "p4 files" which collects details about the specified file(s).  This is less detail than Fstat.
	Files(args ...string) ([]FileDetails, error)

	// Fstat invokes a "p4 fstat" which collects details about the specified file(s).
	Fstat(args ...string) (*FstatResult, error)

	// Grep executes a p4grep and returns details of files and lines matching input pattern.
	// This is designed for small greps and has a limit of 10K files participating in each action.
	Grep(pattern string, caseSensitive bool, depotPaths ...string) ([]Grep, error)

	// GrepLarge operates on a large dataset, and will chunk up the dataset and issue subcalls
	// results of all subcalls are collated and returned via a channel in GrepStatus.
	GrepLarge(pattern string, depotPath string, caseSensitive bool, status *GrepStatus) error

	// Index adds keywords to the p4 index identified by name/attrib.
	Index(name string, attrib int, values ...string) error

	// IndexDelete removes keywords from the p4 index identified by name/attrib.
	IndexDelete(name string, attrib int, values ...string) error

	// KeyGet returns the value of the given key using p4 key.
	// Note: returns "0" and no error if the key doesn't exist.
	KeyGet(key string) (string, error)

	// KeySet sets the value of the given key.
	KeySet(key, val string) error

	// KeyInc increments the given integer key, and returns the new value.
	KeyInc(key string) (string, error)

	// KeyCas does a check-and-set of the value at the specified key.
	// The value is updated to newval iff the current value == oldval,
	// otherwise ErrCasMismatch is returned.
	// Note: this cannot be used on a key that doesn't have a value,
	// so there's still a race condition, and thus it can't be used for
	// true transactions.
	KeyCas(key, oldval, newval string) error

	// Keys returns all key values that match the given pattern
	Keys(pattern string) (map[string]string, error)

	// Have returns all the files and their current revision identified by |patterns| as they are
	// in the client workspace. Equivalent for "p4 have".
	Have(patterns ...string) ([]File, error)

	// Info executes the "p4 info" command which returns details about the current session.
	Info() (*Info, error)

	// Ignores executes the "p4 ignores -i file" command which tells if a file is ignored in P4IGNORE
	Ignores(paths []string) (string, error)

	// Login returns the ticket and expiration for the specified user, or an
	// error.
	Login(user string) (string, time.Time, error)

	// Opened executes the "p4 opened" command, returning info about all locally openend files.
	// change may be an empty string (to include all changes), a CL number, or "default".
	Opened(change string) ([]OpenedFile, error)

	// Print invokes "p4 print" and retrieves specified version(s) of files(s) from the server.
	// Note: though this form will happily retrieve multiple files, all the file
	// contents (along with any info lines if not using -q) will be combined
	// into a single string.
	Print(args ...string) (string, error)

	// PrintEx invokes "p4 print" and retrieves the specified version(s) of file(s) from the server.
	// This variant safely returns multiple files as a map, but doesn't accept
	// any flags.
	PrintEx(files ...string) ([]FileDetails, error)

	// Reconcile invokes "p4 reconcile" and marks the inconsistencies between the workspace and the depot.
	Reconcile(paths []string, cl int) (string, error)

	// Revert invokes "p4 revert" on the given files.
	Revert(paths []string, opts ...string) (string, error)

	// Set invokes "p4 set".
	Set(key, value string) error

	// Sizes invokes "p4 sizes" and returns info about file sizes and counts
	Sizes(dirs ...string) (*SizeCollection, error)

	// Submit submits the given CL.
	Submit(cl int, options ...string) (string, error)

	// Sync performs a sync to the given targets. |options| are passed as is to the command and
	// inserted before the targets. Returns the stdout/stderr of the command.
	//
	// Eg. Sync("//shared/...", "-f") -> p4 sync -f //shared/...
	Sync(targets []string, options ...string) (string, error)

	// SyncSize Gives you the amount of files/bytes that a given sync operation will take given a
	// client setup.  Equivalent to the result of "p4 sync -N".
	// If |targets| is empty, "//..." is assumed.
	SyncSize(targets []string) (*SyncSize, error)

	// Tickets invokes "p4 tickets" and returns a list of open tickets
	Tickets(args ...string) ([]Ticket, error)

	// Trust invokes the `p4 trust` command. |args| are normal arguments you would pass the call.
	Trust(args ...string) error

	// Unshelve performs a "p4 unshelve" command into the default changelist. |cl| will be used for
	// providing the -s flag. If another CL is wanted for the unshelving, you can use |args| to
	// provide the -c option.
	Unshelve(cl int, args ...string) (string, error)

	// Users returns a list of users belonging to current perforce server.
	Users() ([]User, error)

	// VerifiedUnshelve means is that before unshelving the changelist identified with |cl|, the lib
	// will verify that no file is newer within the checkout. This is useful because unshelve will
	// overwrite a newer file, thus stomping any newer changes, which can lead to undesirable
	// situations.
	// This function will error out if any file is in a newer version that the one unshelved.
	// On success, returns stdout of the unshelve.
	VerifiedUnshelve(cl int) (string, error)

	// Where returns the absolute local path that relates to the specified depot path.
	Where(path string) (string, error)

	// WhereEx returns the absolute local paths that relates to the specified depot paths.
	WhereEx(path []string) ([]string, error)

	// Move moves/renames files using p4 move
	Move(cl int, from string, to string) (string, error)
}

// Options -----------------------------------------------------------------------------------------

// Option represents a single option to modify the behaviour of the interface.
// Use the Options functions to add them when creating the interface.
type Option interface {
	apply(*options)
}

// OutputOption determines whether we want the output of |p4.ExecCmd| to be routed here as well as
// the normal return value.
func OutputOption(output io.Writer) Option {
	return fnOption(func(opts *options) {
		opts.output = output
	})
}

type options struct {
	output io.Writer
}

type fnOption func(*options)

func (fn fnOption) apply(opts *options) { fn(opts) }

// P4 Structs --------------------------------------------------------------------------------------

// Client represents all the information associated with a perforce workspace.
// https://www.perforce.com/manuals/v17.1/cmdref/Content/CmdRef/p4_client.html
type Client struct {
	// Required fields.
	Client        string
	Owner         string
	Root          string
	Options       []ClientOption
	SubmitOptions []string
	LineEnd       string
	View          []ViewEntry

	// Optional fields.
	Host           string
	Description    string
	AltRoots       []string
	Stream         string
	StreamAtChange string
	ServerId       string
	ChangeView     []string
}

type ClientOption string

const (
	AllWrite   ClientOption = "allwrite"
	Clobber                 = "clobber"
	Compress                = "compress"
	Locked                  = "locked"
	Modtime                 = "modtime"
	Rmdir                   = "rmdir"
	NoAllWrite              = "noallwrite"
	NoClobber               = "noclobber"
	NoCompress              = "nocompress"
	Unlocked                = "unlocked"
	NoModtime               = "nomodtime"
	NoRmdir                 = "normdir"
)

var clientOptionInverse = map[ClientOption]ClientOption{
	AllWrite:   NoAllWrite,
	NoAllWrite: AllWrite,
	Clobber:    NoClobber,
	NoClobber:  Clobber,
	Compress:   NoCompress,
	NoCompress: Compress,
	Locked:     Unlocked,
	Unlocked:   Locked,
	Modtime:    NoModtime,
	NoModtime:  Modtime,
	Rmdir:      NoRmdir,
	NoRmdir:    Rmdir,
}

// AppendClientOption adds the option a slice. If the option is already there this does nothing.
// If the inverse of the option is already there, it will replace it (eg. if Clobber is already
// present and you add NoClobber, the latter will remain).
func AppendClientOption(options []ClientOption, option ClientOption) ([]ClientOption, error) {
	// We check to see if the option or the inverse are already there.
	inverse, ok := clientOptionInverse[option]
	if !ok {
		return nil, fmt.Errorf("could not find inverse for client option: %v", string(option))
	}
	for i, opt := range options {
		// If we find the option, we don't do anything.
		if opt == option {
			return options, nil
		}
		// If we find the inverse, we replace it.
		if inverse == opt {
			options[i] = option
			return options, nil
		}
	}
	// We didn't find the option of the inverse. We add it.
	options = append(options, option)
	return options, nil
}

func (c *Client) String() string {
	var b strings.Builder
	if c.Client != "" {
		fmt.Fprintf(&b, "Client:\t%s\n", c.Client)
	}
	if c.Owner != "" {
		fmt.Fprintf(&b, "Owner:\t%s\n", c.Owner)
	}
	if c.Host != "" {
		fmt.Fprintf(&b, "Host:\t%s\n", c.Host)
	}
	if c.Root != "" {
		fmt.Fprintf(&b, "Root:\t%s\n", c.Root)
	}
	if len(c.Options) > 0 {
		options := ""
		for _, o := range c.Options {
			options = options + " " + string(o)
		}
		options = strings.TrimSpace(options)
		fmt.Fprintf(&b, "Options:\t%s\n", options)
	}
	if len(c.SubmitOptions) > 0 {
		fmt.Fprintf(&b, "SubmitOptions:\t%s\n", strings.Join(c.SubmitOptions, " "))
	}
	if c.LineEnd != "" {
		fmt.Fprintf(&b, "LineEnd:\t%s\n", c.LineEnd)
	}
	if len(c.AltRoots) > 0 {
		fmt.Fprintf(&b, "AltRoots:\t%s\n", strings.Join(c.AltRoots, " "))
	}
	if c.Stream != "" {
		fmt.Fprintf(&b, "Stream:\t%s\n", c.Stream)
	}
	if c.StreamAtChange != "" {
		fmt.Fprintf(&b, "StreamAtChange:\t%s\n", c.StreamAtChange)
	}
	if c.ServerId != "" {
		fmt.Fprintf(&b, "ServerID:\t%s\n", c.ServerId)
	}
	if len(c.View) > 0 {
		fmt.Fprintf(&b, "View:\n")
		for _, viewEntry := range c.View {
			fmt.Fprintf(&b, "\t%s %s\n", viewEntry.Source, viewEntry.Destination)
		}
	}
	return b.String()
}

// ViewEntry a line within the |view| field of a perforce client.
// See |P4Client| comments for more information.
type ViewEntry struct {
	Source      string
	Destination string
}

// Change stores details about a perforce changelist.
type Change struct {
	Cl          int    `p4:"change"`
	User        string `p4:"user"`
	Client      string `p4:"client"`
	Date        string
	DateUnix    int64  `p4:"time"`
	Description string `p4:"desc"`
	Status      string `p4:"status"`
}

// File represents a file within the depot and (possibly) a local client.
type File struct {
	DepotPath string
	LocalPath string
	Revision  int
}

// FileAction describes action operating on a file at a specific revision.
type FileAction struct {
	DepotPath string `p4:"depotFile"`
	Revision  int    `p4:"rev"`
	Action    string `p4:"action"`
	Type      string `p4:"type"`
	FromFile  string `p4:"fromFile"`
	FromRev   int    `p4:"fromRev"`
	Digest    string `p4:"digest"`
	Size      int    `p4:"fileSize"`
}

// ActionType is a type that enumerates different kinds of file actions
type ActionType int

const (
	ActionAdd = iota
	ActionArchive
	ActionBranch
	ActionDelete
	ActionEdit
	ActionIntegrate
	ActionMoveAdd
	ActionMoveDelete
	ActionPurge

	ActionLen
)

var ActionNames = [...]string{
	"add",
	"archive",
	"branch",
	"delete",
	"edit",
	"integrate",
	"move/add",
	"move/delete",
	"purge",
}

func (at ActionType) String() string {
	if int(at) >= len(ActionNames) || int(at) < 0 {
		return "invalid action"
	}
	return ActionNames[int(at)]
}

func GetActionType(action string) (ActionType, error) {
	for i := range ActionNames {
		if ActionNames[i] == action {
			return ActionType(i), nil
		}
	}
	return ActionLen, fmt.Errorf("couldn't find action %s", action)
}

// FileType is a type that enumerates different kinds of file types
type FileType int

const (
	FileTypeText = iota
	FileTypeBinary
	FileTypeSymlink
	FileTypeApple
	FileTypeResource
	FileTypeUnicode
	FileTypeUtf8
	FileTypeUtf16

	FileTypeLen
)

var FileTypeNames = [...]string{
	"text",
	"binary",
	"symlink",
	"apple",
	"resource",
	"unicode",
	"utf8",
	"utf16",
}

func (ft FileType) String() string {
	if int(ft) >= len(FileTypeNames) || int(ft) < 0 {
		return "invalid file type"
	}
	return FileTypeNames[int(ft)]
}

func GetFileType(fileType string) (FileType, error) {
	for i := range FileTypeNames {
		if FileTypeNames[i] == fileType {
			return FileType(i), nil
		}
	}
	return FileTypeLen, fmt.Errorf("couldn't find file type %s", fileType)
}

// Description is the result of a p4 describe command.
type Description struct {
	Cl          int    `p4:"change"`
	User        string `p4:"user"`
	Client      string `p4:"client"`
	Date        string
	DateUnix    int64        `p4:"time"`
	Description string       `p4:"desc"`
	Status      string       `p4:"status"`
	Shelved     bool         `p4:"shelved"`
	Files       []FileAction `p4:"[depotFile,action,type,rev,digest,fromFile,fromRev]"`
}

// DiffType is a type that enumerates different kinds of file differences.
type DiffType int

const (
	DiffNone = iota
	DiffAdd
	DiffChange
	DiffDelete
	DiffIntegrate
)

func (dt DiffType) String() string {
	switch dt {
	case DiffAdd:
		return "add"
	case DiffChange:
		return "edit"
	case DiffDelete:
		return "delete"
	case DiffIntegrate:
		return "integrate"
	default:
		return "unknown"
	}
}

// Diff contains details about a chunk of difference between two files.
type Diff struct {
	LeftStartLine  int
	LeftEndLine    int
	RightStartLine int
	RightEndLine   int
	DiffType       DiffType
}

// FileDetails contains details about a file.
type FileDetails struct {
	Directory string
	DepotFile string // server path of the file
	Rev       int    // revision number of the file
	Change    int    // changelist number for this revision
	Action    string // last action on this revision
	Type      string // type of the file (text or binary)
	Time      int64  // time of last action on this revision
	FileSize  int    // size of this revision
	Content   []byte
}

// FileStat contains information about a file that exists on the perforce server.
type FileStat struct {
	Action               string   // open action, if opened in workspace
	ActionOwner          string   // user who opened the file
	Change               int      // open changelist number, if open on worksapce
	Charset              string   // charset of open file
	ClientFile           string   // local path to file (in helix syntax with -Op option)
	DepotFile            string   // server path of file
	Digest               string   // MD5 digest of a file
	FileSize             int      // file size in bytes
	HaveRev              int      // version last synced to workspace
	HeadAction           string   // one of add, edit, delete, branch, move/add, move/delete, integrate, import, purge, or archive.
	HeadChange           int      // head revision changelist number
	HeadCharset          string   // charset for unicode files
	HeadModTime          int      // head revision modification time
	HeadRev              int      // head revision number
	HeadTime             int      // head revision changelist time
	HeadType             string   // file type (text,binary,text+k etc)
	IsMapped             bool     // true if file is mapped to local workspace
	MovedFile            string   // depot filename of original file
	MovedRev             int      // head revision of moved file
	OtherActions         []string `p4:"[otherAction]"` // for each user with the file open, the action taken
	OtherChanges         []int    `p4:"[otherChange]"` // list of changelists that other users have this file open
	OtherLock0           bool     // true if another user holds a lock on this file
	OtherLockOwner       string   // user and client holding lock to this file
	OtherOpen            int      // the number of other users that have this file open
	OtherOpens           []string `p4:"[otherOpen]"` // list of users and workspaces that have this file open
	OurLock              bool     // if true, user owns lock on this file
	Path                 string   // local path to file
	Reresolvable         int      // the number of reresolvable integration records
	ResolveActions       []string `p4:"[resolveAction]"`       // list of pending resolve actions
	ResolveBaseFiles     []string `p4:"[resolveBaseFile]"`     // list of base files involved in pending resolve actions
	ResolveBaseRevisions []int    `p4:"[resolveBaseRevision]"` // list of base revisions involved in pending resolve actions
	ResolveFromFiles     []string `p4:"[resolveFromFile]"`     // list of sourcs files involved in pending resolve actions
	ResolveStartFromRevs []int    `p4:"[resolveStartFromRev]"` // list of start revisions involved in pending resolve actions
	ResolveEndFromRevs   []int    `p4:"[resolveEndFromRev]"`   // list of end revisions involved in pending resolve actions
	Resolved             int      // the number of resolved integration records
	Revtime              int
	Shelved              bool   // true if file is shelved
	Type                 string // open type, if opened in worspace (text,binary)
	Unresolved           int    // the number of unresolved integration records
	WorkRev              int    // open revision, if open
}

// FstatResult contains the output of 'p4 fstat' call including summary and list of file details.
type FstatResult struct {
	FileStats []FileStat
	// changelist description if fstat called with -e option
	Desc string `p4:"desc"`
}

// Grep is an atom of grep result.
type Grep struct {
	DepotPath  string
	Revision   int
	LineNumber int
	Contents   string
}

// GrepStatus supplies callee with details about long running greps.
type GrepStatus struct {
	GrepsChan    chan []Grep
	FilesChecked uint64
	BytesChecked uint64
	Total        Size
}

// Info is a set of files specific to a product.
type Info struct {
	User   string
	Client string
	Host   string
	Root   string
}

// Sizes contains details about part of the depot structure.
type Size struct {
	DepotPath string
	FileCount uint64
	FileSize  uint64
}

// Sizecollection contains accumulated results of a series of size operations.
type SizeCollection struct {
	Sizes          []Size
	TotalFileCount uint64
	TotalFileSize  uint64
}

// Status is a type that enumerates different kinds of file status.
type Status int

const (
	StatusNone = iota
	StatusPending
	StatusShelved
	StatusSubmitted
)

// OpenedFile is a P4 depot path + its diff type.
type OpenedFile struct {
	Path   string
	Status ActionType
	CL     int
	Type   FileType
}

// Ticket is a structure detailing perforce tickets.
type Ticket struct {
	Name string
	User string
	ID   string
}

// SyncSize details the size of a sync operation.
type SyncSize struct {
	FilesAdded   int64
	FilesUpdated int64
	FilesDeleted int64
	BytesAdded   int64
	BytesDeleted int64
}

// Users stores details about a perforce user.
type User struct {
	User     string
	Email    string
	Name     string
	Accessed string
}

// UserClient contains a user/client pairing.
type UserClient struct {
	User   string
	Client string
}

// StatsMap holds statistics regarding the execution of commands.
type StatsMap map[string]struct {
	Count   int   // Total number of times the command was executed.
	MinUs   int64 // Minimum execution time for the command (in microseconds).
	MaxUs   int64 // Maximum execution time for the command (in microseconds).
	TotalUs int64 // Total execution time for the command (in microseconds).
}

var Stats = StatsMap{}

// Tracer is used to trace calls to P4.
// Calling the tracer function starts a trace and returns a function to end
// the trace.
type Tracer func(stat string) func()

// Implementation ----------------------------------------------------------------------------------

// Actual implementation struct users can use for real usage.
type impl struct {
	user    string
	passwd  string
	tracer  Tracer
	exePath string
}

func New() P4 {
	return &impl{exePath: "p4"}
}

func NewForUser(user, passwd string) P4 {
	return &impl{user: user, passwd: passwd, exePath: "p4"}
}

// WithTracer attempts to build a new P4 interface with tracing functionality.
// If the provided interface doesn't support tracing, it is returned unchanged.
func WithTracer(p4 P4, tracer Tracer) P4 {
	if parent, ok := p4.(*impl); ok {
		child := *parent
		child.tracer = tracer
		return &child
	}
	return p4
}
