// Package registrytest builds, signs and serves plugin registry indexes for
// tests of the registry client and of the code that uses it. A test registry is
// a TUF repository that a Repo signs and a Server serves.
package registrytest

import (
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/sigstore/sigstore/pkg/signature"
	"github.com/theupdateframework/go-tuf/v2/metadata"
)

// The roles of a TUF repository, as the root names them.
const (
	RoleRoot      = metadata.ROOT
	RoleTargets   = metadata.TARGETS
	RoleSnapshot  = metadata.SNAPSHOT
	RoleTimestamp = metadata.TIMESTAMP
)

// targetName is the one target of a test registry.
const targetName = "index.json"

// farFuture is when the metadata of a repository expires unless a test sets
// another time. It is far enough away that no test depends on the clock.
var farFuture = time.Date(2099, time.January, 1, 0, 0, 0, 0, time.UTC)

// Repo is the publisher of a test registry. It keeps the keys of a TUF
// repository, which are fresh for every Repo, signs the metadata, and holds every
// file it has produced, so that a test can serve any of them in any order.
//
// The repository uses consistent snapshots. The root has 3 root keys with a
// threshold of 2, and one key each for the targets, snapshot and timestamp
// roles. Apart from the timestamp, a file is named after its version, such as
// 2.snapshot.json, and the index is the target targets/<sha256>.index.json.
type Repo struct {
	t testing.TB

	// Keys are the private keys by name, and roleKeys the names of the keys that
	// the root lists for each role. Retired holds the keys that were replaced.
	keys     map[string]ed25519.PrivateKey
	roleKeys map[string][]string
	retired  map[string][]string
	nextKey  int

	root *metadata.Metadata[metadata.RootType]

	// The versions of the metadata that was signed last.
	targets, snapshot, timestamp int64

	expires     map[string]time.Time
	rootSigners int
	signWithOld map[string]bool
	files       map[string][]byte
}

// NewRepo returns a repository with fresh keys and a root of version 1 that
// expires in 2099, like the metadata that it signs afterwards. Nothing is
// published until Server.Publish signs an index.
func NewRepo(t testing.TB) *Repo {
	t.Helper()

	r := &Repo{
		t:           t,
		keys:        make(map[string]ed25519.PrivateKey),
		roleKeys:    make(map[string][]string),
		retired:     make(map[string][]string),
		expires:     make(map[string]time.Time),
		signWithOld: make(map[string]bool),
		files:       make(map[string][]byte),
	}

	for _, role := range []string{RoleRoot, RoleTargets, RoleSnapshot, RoleTimestamp} {
		r.expires[role] = farFuture
	}

	r.root = metadata.Root(farFuture)
	r.root.Signed.ConsistentSnapshot = true
	r.root.Signed.Roles[RoleRoot].Threshold = 2

	for range 3 {
		r.addKey(RoleRoot)
	}

	for _, role := range []string{RoleTargets, RoleSnapshot, RoleTimestamp} {
		r.addKey(role)
	}

	r.signRoot(r.roleKeys[RoleRoot])

	return r
}

// Root returns the first root.json of the repository, which a client trusts as
// the root of its chain of trust, for example as registry.Client.Root.
func (r *Repo) Root() []byte {
	return r.files["1.root.json"]
}

// File returns a file that the repository produced, by its path relative to the
// base URL of the registry, such as "timestamp.json", "2.root.json" or
// "3.snapshot.json". It returns nil when there is no such file. Only the
// timestamp is overwritten by a new version, so a test that wants an older one
// has to keep a copy.
func (r *Repo) File(name string) []byte {
	return r.files[name]
}

// Files returns every file that the repository produced, by path. The caller may
// change the map, but not the files.
func (r *Repo) Files() map[string][]byte {
	return maps.Clone(r.files)
}

// SetExpiry sets when the next piece of metadata of the role that the repository
// signs expires, and the next root when the role is the root. For the root it
// also signs the current root again, with the new time and the same version, so
// call it before Root is used. To set the expiry of the roots that come after the
// current one without touching it, use SetNextRootExpiry.
func (r *Repo) SetExpiry(role string, expires time.Time) {
	r.t.Helper()

	r.expires[role] = expires.UTC()

	if role == RoleRoot {
		r.root.Signed.Expires = expires.UTC()
		r.signRoot(r.roleKeys[RoleRoot])
	}
}

// SetNextRootExpiry sets when the roots that the repository publishes from now on
// expire, and leaves the current root as it is, so a test can build a chain of
// roots whose expiry differs from one version to the next.
func (r *Repo) SetNextRootExpiry(expires time.Time) {
	r.expires[RoleRoot] = expires.UTC()
}

// SignRootWith limits the number of root keys that sign the next roots to n, in
// the order of the keys, where 0 lifts the limit. A root signed by fewer keys
// than the threshold of the root before it must be rejected by a client.
func (r *Repo) SignRootWith(n int) {
	r.rootSigners = n
}

// RotateKey replaces the key of a role with a fresh one and publishes a new root
// of the next version that says so, signed by the root keys. The old key is
// retired, which SignWithRetiredKey can use. For the root role it replaces the
// last of the root keys, so the threshold of the old root is still met by the
// others.
func (r *Repo) RotateKey(role string) {
	r.t.Helper()

	if role == RoleRoot {
		r.ReplaceRootKeys(len(r.roleKeys[RoleRoot]) - 1)

		return
	}

	for _, name := range slices.Clone(r.roleKeys[role]) {
		r.revoke(name, role)
	}

	r.addKey(role)
	r.nextRoot(r.roleKeys[RoleRoot])
}

// ReplaceRootKeys publishes a new root of the next version that keeps the first
// keep root keys and replaces the others with fresh ones. Every key of the new
// set signs it, so its own threshold is met, but only the kept keys count for
// the root before it, so with keep below the threshold of 2 it is what an
// attacker who holds fewer root keys than the threshold would publish.
func (r *Repo) ReplaceRootKeys(keep int) {
	r.t.Helper()

	r.replaceRootKeys(keep, false)
}

// ReplaceRootKeysSignedByOldKeys is ReplaceRootKeys where the new root is signed
// by the root keys that it replaces instead of its own. The threshold of the root
// before it is met, and its own is not, unless keep reaches the threshold.
func (r *Repo) ReplaceRootKeysSignedByOldKeys(keep int) {
	r.t.Helper()

	r.replaceRootKeys(keep, true)
}

// replaceRootKeys replaces the root keys after the first keep, and signs the new
// root with the old keys when old is true and with the new ones otherwise.
func (r *Repo) replaceRootKeys(keep int, old bool) {
	current := slices.Clone(r.roleKeys[RoleRoot])
	for _, name := range current[keep:] {
		r.revoke(name, RoleRoot)
	}

	for range current[keep:] {
		r.addKey(RoleRoot)
	}

	if old {
		r.nextRoot(current)

		return
	}

	r.nextRoot(r.roleKeys[RoleRoot])
}

// SignWithRetiredKey makes the repository sign the next metadata of the role
// with the key that was retired last instead of its current key, which a client
// must reject once the root says that key is no longer the one.
func (r *Repo) SignWithRetiredKey(role string) {
	r.signWithOld[role] = true
}

// SignWithCurrentKeys makes the repository sign with the current keys again.
func (r *Repo) SignWithCurrentKeys() {
	clear(r.signWithOld)
}

// addKey creates a key and adds it to the root for the role.
func (r *Repo) addKey(role string) {
	r.nextKey++

	name := fmt.Sprintf("%s-%d", role, r.nextKey)

	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		r.t.Fatalf("failed to generate a key: %v", err)
	}

	key, err := metadata.KeyFromPublicKey(private.Public())
	if err != nil {
		r.t.Fatalf("failed to convert a key: %v", err)
	}

	if err := r.root.Signed.AddKey(key, role); err != nil {
		r.t.Fatalf("failed to add a key: %v", err)
	}

	r.keys[name] = private
	r.roleKeys[role] = append(r.roleKeys[role], name)
}

// revoke removes a key from the root for the role and retires it.
func (r *Repo) revoke(name, role string) {
	id := r.keyID(name)

	if err := r.root.Signed.RevokeKey(id, role); err != nil {
		r.t.Fatalf("failed to revoke a key: %v", err)
	}

	r.roleKeys[role] = slices.DeleteFunc(r.roleKeys[role], func(other string) bool { return other == name })
	r.retired[role] = append(r.retired[role], name)
}

// keyID returns the TUF key ID of a key.
func (r *Repo) keyID(name string) string {
	key, err := metadata.KeyFromPublicKey(r.keys[name].Public())
	if err != nil {
		r.t.Fatalf("failed to convert a key: %v", err)
	}

	id, err := key.ID()
	if err != nil {
		r.t.Fatalf("failed to work out a key ID: %v", err)
	}

	return id
}

// nextRoot raises the version of the root and signs it with the named keys,
// which are the ones of the old and of the new root that should sign.
func (r *Repo) nextRoot(signers []string) {
	r.root.Signed.Version++
	r.root.Signed.Expires = r.expires[RoleRoot]

	r.signRoot(signers)
}

// signRoot signs the root and stores it under its version, with as many of the
// signers as the limit allows.
func (r *Repo) signRoot(signers []string) {
	if r.rootSigners > 0 && len(signers) > r.rootSigners {
		signers = signers[:r.rootSigners]
	}

	r.files[strconv.FormatInt(r.root.Signed.Version, 10)+".root.json"] = sign(r, r.root, signers)
}

// publish signs the next targets, snapshot and timestamp metadata for an index,
// and stores them with the index as a target file. Every call raises the three
// versions by one.
func (r *Repo) publish(index []byte) {
	r.t.Helper()

	target, err := metadata.TargetFile().FromBytes(targetName, index, "sha256")
	if err != nil {
		r.t.Fatalf("failed to describe the index: %v", err)
	}

	sum := sha256.Sum256(index)

	r.targets++
	signedTargets := metadata.Targets(r.expires[RoleTargets])
	signedTargets.Signed.Version = r.targets
	signedTargets.Signed.Targets = map[string]*metadata.TargetFiles{targetName: target}
	r.files[fmt.Sprintf("%d.targets.json", r.targets)] = sign(r, signedTargets, r.signersOf(RoleTargets))

	r.snapshot++
	signedSnapshot := metadata.Snapshot(r.expires[RoleSnapshot])
	signedSnapshot.Signed.Version = r.snapshot
	signedSnapshot.Signed.Meta["targets.json"] = metadata.MetaFile(r.targets)
	r.files[fmt.Sprintf("%d.snapshot.json", r.snapshot)] = sign(r, signedSnapshot, r.signersOf(RoleSnapshot))

	r.timestamp++
	signedTimestamp := metadata.Timestamp(r.expires[RoleTimestamp])
	signedTimestamp.Signed.Version = r.timestamp
	signedTimestamp.Signed.Meta["snapshot.json"] = metadata.MetaFile(r.snapshot)
	r.files["timestamp.json"] = sign(r, signedTimestamp, r.signersOf(RoleTimestamp))

	r.files["targets/"+hex.EncodeToString(sum[:])+"."+targetName] = index
}

// signersOf returns the names of the keys that sign the metadata of a role.
func (r *Repo) signersOf(role string) []string {
	if r.signWithOld[role] && len(r.retired[role]) > 0 {
		return r.retired[role][len(r.retired[role])-1:]
	}

	return r.roleKeys[role]
}

// sign replaces the signatures of the metadata with those of the named keys and
// returns the JSON of the metadata.
func sign[T metadata.Roles](r *Repo, meta *metadata.Metadata[T], signers []string) []byte {
	r.t.Helper()

	meta.ClearSignatures()

	for _, name := range signers {
		signer, err := signature.LoadSigner(r.keys[name], crypto.Hash(0))
		if err != nil {
			r.t.Fatalf("failed to load the key %s: %v", name, err)
		}

		if _, err := meta.Sign(signer); err != nil {
			r.t.Fatalf("failed to sign with the key %s: %v", name, err)
		}
	}

	data, err := meta.ToBytes(true)
	if err != nil {
		r.t.Fatalf("failed to encode the metadata: %v", err)
	}

	return data
}
