package git

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"os"
	"strings"

	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/plumbing/object"
	"github.com/hiddeco/sshsig"
	gossh "golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	. "github.com/jig/lisp/types"
)

// gitNamespace is the sshsig namespace git uses for commit and tag
// signatures (see gitformat-signature).
const gitNamespace = "git"

// signerSource, when set, supplies candidate signers instead of the
// ssh-agent. It is a Go-embedder / test seam: signing itself only ever
// happens when lisp code passes :sign with an allowed-signers list,
// and the supplied signer must still match that list. No private key
// ever enters the lisp side.
var signerSource func() ([]gossh.Signer, error)

// SetSigner installs a signer source of one (a Go embedder API — e.g.
// a crypto.Signer adapted with gossh.NewSignerFromSigner — and the
// test seam). It does not by itself sign anything: each git-commit /
// git-tag call still decides with :sign, and the signer's public key
// must be listed there.
func SetSigner(resolve func() (gossh.Signer, error)) {
	signerSource = func() ([]gossh.Signer, error) {
		s, err := resolve()
		if err != nil {
			return nil, err
		}
		return []gossh.Signer{s}, nil
	}
}

// ClearSigner removes the signer source (resolution falls back to the
// ssh-agent). Used by tests to reset state.
func ClearSigner() { signerSource = nil }

// resolveSigner finds a signer whose public key is listed in
// allowedKeys: from the installed signer source when present, from the
// ssh-agent (SSH_AUTH_SOCK) otherwise. Fails closed when nothing
// matches.
func resolveSigner(allowedKeys string) (gossh.Signer, error) {
	if signerSource != nil {
		signers, err := signerSource()
		if err != nil {
			return nil, err
		}
		for _, s := range signers {
			if keyListed(s.PublicKey(), allowedKeys) {
				return s, nil
			}
		}
		return nil, fmt.Errorf("no registered signer is listed in the :sign keys")
	}
	return resolveAgentSigner(os.Getenv("SSH_AUTH_SOCK"), allowedKeys)
}

// resolveAgentSigner finds the ssh-agent signer whose public key is
// listed in allowedKeys. The agent connection is kept open for the
// process lifetime because the returned signer calls back over it.
func resolveAgentSigner(sshAuthSock, allowedKeys string) (gossh.Signer, error) {
	if sshAuthSock == "" {
		return nil, fmt.Errorf(":sign needs an ssh-agent (or a Go-registered signer), but SSH_AUTH_SOCK is unset")
	}
	conn, err := net.Dial("unix", sshAuthSock)
	if err != nil {
		return nil, fmt.Errorf("cannot reach ssh-agent at %s: %w", sshAuthSock, err)
	}
	signers, err := agent.NewClient(conn).Signers()
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("ssh-agent: %w", err)
	}
	for _, s := range signers {
		if keyListed(s.PublicKey(), allowedKeys) {
			return s, nil
		}
	}
	_ = conn.Close()
	return nil, fmt.Errorf("no ssh-agent key is listed in the :sign keys")
}

// keyListed reports whether pub appears in allowedKeys (authorized_keys
// format; blank lines and # comments skipped, options rejected as in
// matchAllowedKey).
func keyListed(pub gossh.PublicKey, allowedKeys string) bool {
	want := pub.Marshal()
	for line := range strings.SplitSeq(allowedKeys, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		p, _, options, _, err := gossh.ParseAuthorizedKey([]byte(line))
		if err != nil || len(options) > 0 {
			continue
		}
		if bytes.Equal(p.Marshal(), want) {
			return true
		}
	}
	return false
}

// payloadEncoder is the part of commits and tags that reproduces the exact
// bytes a signature covers.
type payloadEncoder interface {
	EncodeWithoutSignature(o plumbing.EncodedObject) error
}

// signedPayload returns a reader over the canonical signed bytes of a
// commit or tag object.
func signedPayload(obj payloadEncoder) (io.Reader, error) {
	mem := &plumbing.MemoryObject{}
	if err := obj.EncodeWithoutSignature(mem); err != nil {
		return nil, err
	}
	return mem.Reader()
}

// armorSign signs the object's payload and returns the armored SSH
// signature. SHA-512 is what git and ssh-keygen -Y sign use by default.
func armorSign(obj payloadEncoder, signer gossh.Signer) (string, error) {
	rd, err := signedPayload(obj)
	if err != nil {
		return "", err
	}
	sig, err := sshsig.Sign(rd, signer, sshsig.HashSHA512, gitNamespace)
	if err != nil {
		return "", err
	}
	return string(sshsig.Armor(sig)), nil
}

// resignCommit re-creates the commit at hash with an SSH signature in the
// header matching the repo's object format (gpgsig-sha256 for sha256
// repos, gpgsig otherwise) and moves the HEAD branch to the new commit,
// like an amend. go-git's CommitOptions.Signer is not used because it
// always writes gpgsig, which git ignores in sha256 repositories.
func resignCommit(r *Repo, hash plumbing.Hash, signer gossh.Signer) (plumbing.Hash, error) {
	c, err := r.repo.CommitObject(hash)
	if err != nil {
		return plumbing.ZeroHash, err
	}

	armored, err := armorSign(c, signer)
	if err != nil {
		return plumbing.ZeroHash, err
	}
	if r.sha256 {
		c.SignatureSHA256 = armored
	} else {
		c.Signature = armored
	}
	obj := r.repo.Storer.NewEncodedObject()
	if err := c.Encode(obj); err != nil {
		return plumbing.ZeroHash, err
	}
	signedHash, err := r.repo.Storer.SetEncodedObject(obj)
	if err != nil {
		return plumbing.ZeroHash, err
	}
	head, err := r.repo.Head()
	if err != nil {
		return plumbing.ZeroHash, err
	}
	ref := plumbing.NewHashReference(head.Name(), signedHash)
	return signedHash, r.repo.Storer.SetReference(ref)
}

// resignTag is resignCommit for annotated tags: it re-creates the tag
// object with the signature and repoints the tag reference. Unlike
// commits, tag signatures are appended to the body in both sha1 and
// sha256 repositories, so the canonical Signature field is always used.
func resignTag(r *Repo, ref *plumbing.Reference, signer gossh.Signer) (*plumbing.Reference, error) {
	tag, err := r.repo.TagObject(ref.Hash())
	if err != nil {
		return nil, err
	}
	armored, err := armorSign(tag, signer)
	if err != nil {
		return nil, err
	}
	tag.Signature = armored
	obj := r.repo.Storer.NewEncodedObject()
	if err := tag.Encode(obj); err != nil {
		return nil, err
	}
	signedHash, err := r.repo.Storer.SetEncodedObject(obj)
	if err != nil {
		return nil, err
	}
	signedRef := plumbing.NewHashReference(ref.Name(), signedHash)
	return signedRef, r.repo.Storer.SetReference(signedRef)
}

func gitVerifyCommit(rv MalType, rev string, allowedKeys string) (MalType, error) {
	r, err := asRepo("git-verify-commit", rv)
	if err != nil {
		return nil, err
	}
	c, err := commitAt(r, rev)
	if err != nil {
		return nil, err
	}
	armored := c.SignatureSHA256
	if armored == "" {
		armored = c.Signature
	}
	if armored == "" {
		return nil, fmt.Errorf("git-verify-commit: commit %s is not signed", c.Hash)
	}
	return verifySignature(c, armored, allowedKeys)
}

func gitVerifyTag(rv MalType, name string, allowedKeys string) (MalType, error) {
	r, err := asRepo("git-verify-tag", rv)
	if err != nil {
		return nil, err
	}
	ref, err := r.repo.Reference(plumbing.NewTagReferenceName(name), true)
	if err != nil {
		return nil, err
	}
	tag, err := r.repo.TagObject(ref.Hash())
	if err != nil {
		return nil, fmt.Errorf("git-verify-tag: %q is not an annotated tag", name)
	}
	armored := tag.SignatureSHA256
	if armored == "" {
		armored = tag.Signature
	}
	if armored == "" {
		return nil, fmt.Errorf("git-verify-tag: tag %q is not signed", name)
	}
	return verifySignature(tag, armored, allowedKeys)
}

// VerifyCommitSSH checks that commit c carries an SSH signature made by
// one of the allowed public keys (authorized_keys-format lines, as
// git-verify-commit) and returns the matching key's comment and its
// SHA256 fingerprint. Exported for the integrity mode of the CLI.
func VerifyCommitSSH(c *object.Commit, allowedKeys string) (signer, fingerprint string, err error) {
	armored := c.SignatureSHA256
	if armored == "" {
		armored = c.Signature
	}
	if armored == "" {
		return "", "", fmt.Errorf("commit %s is not signed", c.Hash)
	}
	v, err := verifySignature(c, armored, allowedKeys)
	if err != nil {
		return "", "", err
	}
	signer, fingerprint = signerOf(v)
	return signer, fingerprint, nil
}

// VerifyTagSSH is VerifyCommitSSH for annotated tag objects.
func VerifyTagSSH(tag *object.Tag, allowedKeys string) (signer, fingerprint string, err error) {
	armored := tag.SignatureSHA256
	if armored == "" {
		armored = tag.Signature
	}
	if armored == "" {
		return "", "", fmt.Errorf("tag %q is not signed", tag.Name)
	}
	v, err := verifySignature(tag, armored, allowedKeys)
	if err != nil {
		return "", "", err
	}
	signer, fingerprint = signerOf(v)
	return signer, fingerprint, nil
}

// signerOf extracts the :signer comment and :fingerprint from a
// verifySignature result.
func signerOf(v MalType) (signer, fingerprint string) {
	m, ok := v.(HashMap)
	if !ok {
		return "", ""
	}
	s, _ := m.Items[KW("signer")].(string)
	f, _ := m.Items[KW("fingerprint")].(string)
	return s, f
}

// verifySignature checks the armored SSH signature of a commit or tag
// against the allowed public keys. It fails closed: a value is returned
// only when a listed key produced a valid signature over the object's
// exact payload.
func verifySignature(obj payloadEncoder, armored, allowedKeys string) (MalType, error) {
	sig, err := sshsig.Unarmor([]byte(armored))
	if err != nil {
		return nil, err
	}
	pub, comment, err := matchAllowedKey(sig, allowedKeys)
	if err != nil {
		return nil, err
	}
	rd, err := signedPayload(obj)
	if err != nil {
		return nil, err
	}
	if err := sshsig.Verify(rd, sig, pub, sig.HashAlgorithm, gitNamespace); err != nil {
		return nil, err
	}
	return HashMap{Items: map[MalType]MalType{
		KW("valid"):          true,
		KW("key-type"):       pub.Type(),
		KW("fingerprint"):    gossh.FingerprintSHA256(pub),
		KW("hash-algorithm"): string(sig.HashAlgorithm),
		KW("signer"):         comment,
	}}, nil
}

// matchAllowedKey finds the allowed key (authorized_keys-format lines;
// blank lines and # comments skipped) matching the signature's public
// key. The format is authorized_keys / a `.pub` file — one key per line,
// `<type> <base64> [comment]` — NOT git's allowed_signers (which puts
// the principal first). A principal-first line is rejected rather than
// silently misparsed: ParseAuthorizedKey would read the principal as an
// SSH option and drop it, verifying the key with no identity or
// validity constraint. Options are meaningless for signature checking,
// so any line carrying them is refused.
func matchAllowedKey(sig *sshsig.Signature, allowedKeys string) (gossh.PublicKey, string, error) {
	want := sig.PublicKey.Marshal()
	for line := range strings.SplitSeq(allowedKeys, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		pub, comment, options, _, err := gossh.ParseAuthorizedKey([]byte(line))
		if err != nil {
			return nil, "", fmt.Errorf("invalid allowed key %q: %w (expected an authorized_keys / .pub line, not git allowed_signers)", line, err)
		}
		if len(options) > 0 {
			return nil, "", fmt.Errorf("allowed key %q carries unsupported options %q; expected a plain authorized_keys / .pub line (git allowed_signers, principal-first, is not supported)", line, options)
		}
		if bytes.Equal(pub.Marshal(), want) {
			return pub, comment, nil
		}
	}
	return nil, "", fmt.Errorf("no allowed public key matches the signature")
}
