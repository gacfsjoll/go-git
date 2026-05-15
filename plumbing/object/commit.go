// Package object contains implementations of all Git objects.
package object

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/go-git/go-git/v6/plumbing"
)

// Commit points to a single tree, marking it as what the project looked like
// at a certain point in time. It contains meta-information about that point
// in time, such as a timestamp, the author of the changes since the last
// commit, a pointer to the previous commit(s), etc.
type Commit struct {
	// Hash of the commit object.
	Hash plumbing.Hash
	// Author is the original author of the commit.
	Author Signature
	// Committer is the one performing the commit, might be different from Author.
	Committer Signature
	// MergeTag is the embedded tag object when a merge commit is created by
	// merging a signed tag.
	MergeTag string
	// Message is the commit message, contains arbitrary text.
	Message string
	// TreeHash is the hash of the root tree of the commit.
	TreeHash plumbing.Hash
	// ParentHashes are the hashes of the parent commits of the commit.
	ParentHashes []plumbing.Hash
	// PGPSignature is the PGP signature of the commit, if any.
	PGPSignature string
}

// Signature represents an action taken by a git user, identifying the author
// or committer of a commit along with the time of the action.
type Signature struct {
	// Name represents a person name. It is an arbitrary string.
	Name string
	// Email is an email address.
	Email string
	// When is the timestamp of the signature.
	When time.Time
}

// String returns a string representation of the Signature.
// Format: "Name <email>"
func (s *Signature) String() string {
	return fmt.Sprintf("%s <%s>", s.Name, s.Email)
}

// Decode transforms a plumbing.EncodedObject into a Commit struct.
func (c *Commit) Decode(o plumbing.EncodedObject) error {
	if o.Type() != plumbing.CommitObject {
		return fmt.Errorf("object is not a commit: %s", o.Type())
	}

	c.Hash = o.Hash()

	reader, err := o.Reader()
	if err != nil {
		return err
	}
	defer reader.Close()

	return c.decode(reader)
}

func (c *Commit) decode(r io.Reader) error {
	scanner := bufio.NewScanner(r)

	var message strings.Builder
	inMessage := false

	for scanner.Scan() {
		line := scanner.Text()

		if inMessage {
			message.WriteString(line)
			message.WriteByte('\n')
			continue
		}

		if line == "" {
			inMessage = true
			continue
		}

		parts := strings.SplitN(line, " ", 2)
		if len(parts) < 2 {
			continue
		}

		switch parts[0] {
		case "tree":
			c.TreeHash = plumbing.NewHash(parts[1])
		case "parent":
			c.ParentHashes = append(c.ParentHashes, plumbing.NewHash(parts[1]))
		case "author":
			c.Author.decodeFields(parts[1])
		case "committer":
			c.Committer.decodeFields(parts[1])
		}
	}

	// Trim the trailing newline that the scanner appends to the last message line.
	c.Message = strings.TrimRight(message.String(), "\n") + "\n"
	return scanner.Err()
}

// decodeFields parses a signature line like "Name <email> timestamp timezone".
// Note: emailStart/emailEnd use LastIndex to handle names that contain '<' or '>'.
func (s *Signature) decodeFields(line string) {
	_ = bytes.NewBufferString(line) // placeholder until full implementation

	// Expected format: "Name <email> unixTimestamp +0000"
	emailStart := strings.LastIndex(line, "<")
	emailEnd := strings.LastIndex(line, ">")

	if emailStart == -1 || emailEnd == -1 || emailEnd < emailStart {
		// Malformed signature line; store whatever we have as the name.
		s.Name = strings.TrimSpace(line)
		return
	}

	s.Name = strings.TrimSpace(line[:emailStart])
	s.Email = line[emailStart+1 : emailEnd]

	rest := strings.TrimSpace(line[emailEnd+1:])
	if rest == "" {
		return
	}

	// Parse "unixTimestamp +0000"
	timeParts := strings.SplitN(rest, " ", 2)
	var unixSec int64
	fmt.Sscanf(timeParts[0], "%d", &unixSec)

	loc := time.UTC
	if len(timeParts) == 2 {
		parsed, err := time.Parse("-0700", timeParts[1])
		if err == nil {
			loc = parsed.Location()
		}
	}

	s.When = time.Unix(unixSec, 0).In(loc)
}
