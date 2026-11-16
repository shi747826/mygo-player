package plugins

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"sync"
)

type Runtime struct {
	Type            string   `json:"type"`
	ProtocolVersion int      `json:"protocolVersion"`
	Executable      string   `json:"executable"`
	Args            []string `json:"args,omitempty"`
}
type Option struct {
	Label string `json:"label"`
	Value string `json:"value"`
}
type Navigation = Option
type Filter struct {
	Label   string   `json:"label"`
	Value   string   `json:"value"`
	Options []Option `json:"options"`
}
type Manifest struct {
	ID             string       `json:"id"`
	Name           string       `json:"name"`
	Version        string       `json:"version"`
	Runtime        Runtime      `json:"runtime"`
	Capabilities   []string     `json:"capabilities"`
	Navigation     []Navigation `json:"navigation"`
	Filters        []Filter     `json:"filters"`
	DefaultEnabled bool         `json:"defaultEnabled,omitempty"`
}

func (m Manifest) Has(capability string) bool {
	for _, c := range m.Capabilities {
		if c == capability {
			return true
		}
	}
	return false
}

var slug = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

func validRelative(p string) bool {
	return p != "" && !strings.Contains(p, "\\") && !filepath.IsAbs(p) && filepath.VolumeName(p) == "" && filepath.Clean(p) == p && p != "." && p != ".." && !strings.HasPrefix(p, ".."+string(filepath.Separator))
}
func ParseManifest(raw []byte) (Manifest, error) {
	var m Manifest
	if len(raw) > 64<<10 {
		return m, errors.New("plugin manifest too large")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&m); err != nil {
		return m, err
	}
	if d.Decode(new(any)) != io.EOF {
		return m, errors.New("trailing manifest data")
	}
	return m, m.Validate()
}
func (m Manifest) Validate() error {
	if !slug.MatchString(m.ID) || len(m.ID) > 128 || strings.TrimSpace(m.Name) == "" || strings.TrimSpace(m.Version) == "" {
		return errors.New("invalid plugin identity")
	}
	if m.Runtime.Type != "exec" || m.Runtime.ProtocolVersion != ProtocolVersion || !validRelative(m.Runtime.Executable) {
		return errors.New("unsupported plugin runtime")
	}
	allowed := map[string]bool{"detail": true, "resolve": true, "discover": true, "search": true, "rank": true, "seasons": true, "covers": true, "calendar": true, "resetDevice": true, "danmaku": true, "account": true, "favoritesSync": true, "reservations": true}
	seen := map[string]bool{}
	for _, c := range m.Capabilities {
		if !allowed[c] || seen[c] {
			return fmt.Errorf("invalid capability %q", c)
		}
		seen[c] = true
	}
	if !seen["detail"] || !seen["resolve"] {
		return errors.New("detail and resolve capabilities are required")
	}
	nav := map[string]bool{}
	for _, n := range m.Navigation {
		if strings.TrimSpace(n.Label) == "" || strings.TrimSpace(n.Value) == "" || nav[n.Value] {
			return errors.New("invalid plugin navigation")
		}
		nav[n.Value] = true
	}
	filters := map[string]bool{}
	for _, f := range m.Filters {
		if f.Label == "" || (f.Value != "channel" && f.Value != "sort") || filters[f.Value] || len(f.Options) == 0 {
			return errors.New("invalid plugin filter")
		}
		filters[f.Value] = true
		values := map[string]bool{}
		for _, o := range f.Options {
			if o.Label == "" || values[o.Value] {
				return errors.New("invalid plugin filter option")
			}
			values[o.Value] = true
		}
	}
	return nil
}
func readLimited(path string, max int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, max+1))
	if int64(len(b)) > max {
		return nil, errors.New("plugin file too large")
	}
	return b, err
}

// safeExecutable rejects symlinks in every path component.
func safeExecutable(root, relative string) (string, error) {
	if !validRelative(relative) {
		return "", ErrProtocol
	}
	p := root
	for _, part := range strings.Split(relative, string(filepath.Separator)) {
		p = filepath.Join(p, part)
		st, err := os.Lstat(p)
		if err != nil {
			return "", err
		}
		if st.Mode()&os.ModeSymlink != 0 {
			return "", errors.New("plugin symlink is forbidden")
		}
	}
	st, err := os.Stat(p)
	if err != nil {
		return "", err
	}
	if !st.Mode().IsRegular() {
		return "", errors.New("plugin executable is not a regular file")
	}
	return p, nil
}

type Manager struct {
	mu        sync.Mutex
	dir       string
	manifests map[string]Manifest
	paths     map[string]string
	bundled   map[string]bool
	enabled   map[string]bool
	clients   map[string]*Client
}

func LoadManager(dataDir string) (*Manager, error) {
	if strings.TrimSpace(dataDir) == "" {
		return nil, errors.New("plugin: data directory is required")
	}
	dataDir, err := filepath.Abs(dataDir)
	if err != nil {
		return nil, err
	}
	m := &Manager{dir: dataDir, enabled: map[string]bool{}, clients: map[string]*Client{}}
	b, err := readLimited(filepath.Join(dataDir, "plugins", "enabled.json"), 64<<10)
	if err == nil {
		if err = json.Unmarshal(b, &m.enabled); err != nil {
			return nil, err
		}
		if m.enabled == nil {
			m.enabled = map[string]bool{}
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	if err = m.Reload(); err != nil {
		return nil, err
	}
	return m, nil
}
func bundlePaths() []string {
	exe, err := os.Executable()
	if err != nil {
		return nil
	}
	base := filepath.Dir(exe)
	return []string{filepath.Join(base, "plugins"), filepath.Clean(filepath.Join(base, "..", "Resources", "plugins"))}
}

// Reload discovers packages without starting any processes. User packages override bundled IDs.
func (m *Manager) Reload() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	manifests := map[string]Manifest{}
	paths := map[string]string{}
	bundled := map[string]bool{}
	roots := append(bundlePaths(), filepath.Join(m.dir, "plugins"))
	for i, root := range roots {
		entries, err := os.ReadDir(root)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		for _, e := range entries {
			if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
				continue
			}
			p := filepath.Join(root, e.Name())
			manifestPath, err := safeExecutable(p, "plugin.json")
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			if err != nil {
				return err
			}
			b, err := readLimited(manifestPath, 64<<10)
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			if err != nil {
				return err
			}
			v, err := ParseManifest(b)
			if err != nil {
				return fmt.Errorf("%s: %w", p, err)
			}
			if v.ID != e.Name() {
				return errors.New("plugin directory and ID differ")
			}
			if _, err = safeExecutable(p, v.Runtime.Executable); err != nil {
				return fmt.Errorf("%s: %w", p, err)
			}
			manifests[v.ID] = v
			paths[v.ID] = p
			bundled[v.ID] = i < len(roots)-1
		}
	}
	for id, c := range m.clients {
		if v, ok := manifests[id]; !ok || m.paths[id] != paths[id] || !reflect.DeepEqual(m.manifests[id], v) {
			c.Close()
			delete(m.clients, id)
		}
	}
	for id, v := range manifests {
		if _, ok := m.enabled[id]; !ok {
			m.enabled[id] = bundled[id] && v.DefaultEnabled
		}
	}
	m.manifests = manifests
	m.paths = paths
	m.bundled = bundled
	return nil
}
func (m *Manager) Manifests() []Manifest {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Manifest, 0, len(m.manifests))
	for _, v := range m.manifests {
		b, _ := json.Marshal(v)
		var clone Manifest
		_ = json.Unmarshal(b, &clone)
		out = append(out, clone)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
func (m *Manager) Enabled(id string) bool   { m.mu.Lock(); defer m.mu.Unlock(); return m.enabled[id] }
func (m *Manager) IsBundled(id string) bool { m.mu.Lock(); defer m.mu.Unlock(); return m.bundled[id] }
func (m *Manager) saveEnabled() error {
	b, err := json.MarshalIndent(m.enabled, "", "  ")
	if err != nil {
		return err
	}
	root := filepath.Join(m.dir, "plugins")
	if err = os.MkdirAll(root, 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(root, ".enabled-")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if _, err = f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(name, filepath.Join(root, "enabled.json"))
}
func (m *Manager) SetEnabled(id string, enabled bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.manifests[id]; !ok {
		return ErrUnsupported
	}
	old := m.enabled[id]
	m.enabled[id] = enabled
	if err := m.saveEnabled(); err != nil {
		m.enabled[id] = old
		return err
	}
	if !enabled {
		if c := m.clients[id]; c != nil {
			c.Close()
			delete(m.clients, id)
		}
	}
	return nil
}
func (m *Manager) Open(ctx context.Context, id string) (*Client, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.manifests[id]
	if !ok {
		return nil, ErrUnsupported
	}
	if !m.enabled[id] {
		return nil, fmt.Errorf("plugin %q is disabled", id)
	}
	if c := m.clients[id]; c != nil {
		select {
		case <-c.done:
			delete(m.clients, id)
		default:
			return c, nil
		}
	}
	c, err := startPlugin(ctx, v, m.paths[id], m.dir)
	if err == nil {
		m.clients[id] = c
	}
	return c, err
}
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, c := range m.clients {
		c.Close()
		delete(m.clients, id)
	}
	return nil
}

const MaxPackageSize int64 = 256 << 20

// Install copies a directory or ZIP into the user package directory. It never runs or enables it.
func (m *Manager) Install(path string) (Manifest, error) {
	var zero Manifest
	root := filepath.Join(m.dir, "plugins")
	if err := os.MkdirAll(root, 0700); err != nil {
		return zero, err
	}
	stage, err := os.MkdirTemp(root, ".install-")
	if err != nil {
		return zero, err
	}
	defer os.RemoveAll(stage)
	st, err := os.Lstat(path)
	if err != nil {
		return zero, err
	}
	var total int64
	entries := 0
	copyFile := func(rel string, mode fs.FileMode, r io.Reader, size int64) error {
		entries++
		if entries > 10000 {
			return errors.New("too many plugin package entries")
		}
		if !validRelative(rel) || mode&os.ModeSymlink != 0 || !mode.IsRegular() {
			return errors.New("unsafe plugin package entry")
		}
		if size < 0 || size > MaxPackageSize-total {
			return errors.New("plugin package exceeds size limit")
		}
		total += size
		dest := filepath.Join(stage, rel)
		if err := os.MkdirAll(filepath.Dir(dest), 0700); err != nil {
			return err
		}
		f, err := os.OpenFile(dest, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode.Perm()|0600)
		if err != nil {
			return err
		}
		n, e := io.Copy(f, io.LimitReader(r, size+1))
		ce := f.Close()
		if e != nil {
			return e
		}
		if n != size {
			return errors.New("plugin entry size mismatch")
		}
		return ce
	}
	if st.IsDir() {
		err = filepath.WalkDir(path, func(p string, d fs.DirEntry, e error) error {
			if e != nil {
				return e
			}
			if p == path {
				return nil
			}
			rel, e := filepath.Rel(path, p)
			if e != nil {
				return e
			}
			info, e := d.Info()
			if e != nil {
				return e
			}
			if info.Mode()&os.ModeSymlink != 0 {
				return errors.New("plugin symlink is forbidden")
			}
			if d.IsDir() {
				return nil
			}
			f, e := os.Open(p)
			if e != nil {
				return e
			}
			defer f.Close()
			return copyFile(rel, info.Mode(), f, info.Size())
		})
	} else if st.Mode().IsRegular() {
		z, e := zip.OpenReader(path)
		if e != nil {
			return zero, e
		}
		defer z.Close()
		if len(z.File) > 10000 {
			return zero, errors.New("too many plugin package entries")
		}
		for _, f := range z.File {
			rel := filepath.FromSlash(f.Name)
			if f.Mode()&os.ModeSymlink != 0 {
				err = errors.New("plugin symlink is forbidden")
				break
			}
			if f.FileInfo().IsDir() {
				if !validRelative(strings.TrimSuffix(rel, string(filepath.Separator))) {
					err = errors.New("unsafe plugin directory")
					break
				}
				continue
			}
			if f.UncompressedSize64 > uint64(MaxPackageSize) {
				err = errors.New("plugin package exceeds size limit")
				break
			}
			r, e := f.Open()
			if e != nil {
				err = e
				break
			}
			err = copyFile(rel, f.Mode(), r, int64(f.UncompressedSize64))
			r.Close()
			if err != nil {
				break
			}
		}
	} else {
		err = errors.New("plugin path must be a directory or ZIP")
	}
	if err != nil {
		return zero, err
	}
	b, err := readLimited(filepath.Join(stage, "plugin.json"), 64<<10)
	if err != nil {
		return zero, err
	}
	v, err := ParseManifest(b)
	if err != nil {
		return zero, err
	}
	exe, err := safeExecutable(stage, v.Runtime.Executable)
	if err != nil {
		return zero, err
	}
	if err = os.Chmod(exe, 0700); err != nil {
		return zero, err
	}
	m.mu.Lock()
	dest := filepath.Join(root, v.ID)
	if _, err = os.Lstat(dest); err == nil {
		m.mu.Unlock()
		return zero, errors.New("plugin already installed")
	}
	if !errors.Is(err, fs.ErrNotExist) {
		m.mu.Unlock()
		return zero, err
	}
	err = os.Rename(stage, dest)
	if err == nil {
		if c := m.clients[v.ID]; c != nil {
			c.Close()
			delete(m.clients, v.ID)
		}
		m.enabled[v.ID] = false
		err = m.saveEnabled()
	}
	m.mu.Unlock()
	if err != nil {
		return zero, err
	}
	if err = m.Reload(); err != nil {
		return zero, err
	}
	return v, nil
}

// Uninstall removes only a user package, retaining its separate plugin data directory.
func (m *Manager) Uninstall(id string) error {
	m.mu.Lock()
	if !slug.MatchString(id) {
		m.mu.Unlock()
		return ErrUnsupported
	}
	p := filepath.Join(m.dir, "plugins", id)
	st, err := os.Lstat(p)
	if err != nil {
		m.mu.Unlock()
		return err
	}
	if !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
		m.mu.Unlock()
		return errors.New("unsafe plugin directory")
	}
	if c := m.clients[id]; c != nil {
		c.Close()
		delete(m.clients, id)
	}
	// id is validated and p is a direct child of the fixed user plugins root.
	if err = os.RemoveAll(p); err == nil {
		delete(m.enabled, id)
		err = m.saveEnabled()
	}
	m.mu.Unlock()
	if err != nil {
		return err
	}
	return m.Reload()
}
