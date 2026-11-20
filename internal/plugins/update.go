package plugins

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const pluginUpdateSourceFile = ".mygo-update-source"

var ErrNoPluginUpdate = errors.New("plugin: no newer version available")

func (l githubLink) updateSource() string {
	root := "https://github.com/" + l.owner + "/" + l.repo
	if l.asset != "" {
		return root + "/releases/latest/download/" + l.asset
	}
	return root
}

func writeUpdateSource(dir, source string) error {
	if source == "" {
		return nil
	}
	return os.WriteFile(filepath.Join(dir, pluginUpdateSourceFile), []byte(source), 0600)
}

// UpdateSource returns the GitHub source that was verified when the package was
// installed or updated. Legacy and manually installed packages can have no source.
func (m *Manager) UpdateSource(id string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.updateSourceLocked(id)
}

func (m *Manager) updateSourceLocked(id string) string {
	if m.paths[id] == "" || m.bundled[id] {
		return ""
	}
	path, err := safeExecutable(m.paths[id], pluginUpdateSourceFile)
	if err != nil {
		return ""
	}
	data, err := readLimited(path, 4096)
	if err != nil {
		return ""
	}
	link, err := parseGitHubLink(string(data))
	if err != nil {
		return ""
	}
	return link.updateSource()
}

// UpdateFrom uses an existing local package, an explicitly supplied GitHub URL,
// or the installed package's saved GitHub source if input is empty.
func (m *Manager) UpdateFrom(ctx context.Context, id, input string) (Manifest, error) {
	return m.UpdateFromWithProgress(ctx, id, input, nil)
}

func (m *Manager) UpdateFromWithProgress(ctx context.Context, id, input string, progress func(string)) (Manifest, error) {
	return newGitHubInstaller().update(ctx, m, id, input, progress)
}

func (g githubInstaller) update(ctx context.Context, m *Manager, id, input string, progress func(string)) (Manifest, error) {
	m.mu.Lock()
	_, installed := m.manifests[id]
	source := m.updateSourceLocked(id)
	m.mu.Unlock()
	if !installed {
		return Manifest{}, ErrUnsupported
	}
	input = strings.TrimSpace(input)
	useSavedSource := input == ""
	if useSavedSource {
		if source == "" {
			return Manifest{}, errors.New("plugin: provide a GitHub release link or a local plugin package")
		}
		input = source
	} else if !strings.Contains(input, "://") && !strings.HasPrefix(input, "github.com/") {
		return m.updatePackage(ctx, id, input, "", "")
	}
	link, err := parseGitHubLink(input)
	if err != nil {
		return Manifest{}, err
	}
	var expectedSource string
	if useSavedSource {
		expectedSource = source
	}
	return g.withPackage(ctx, link, useSavedSource, progress, func(ctx context.Context, path string) (Manifest, error) {
		return m.updatePackage(ctx, id, path, link.updateSource(), expectedSource)
	})
}

func (m *Manager) updatePackage(ctx context.Context, id, path, source, expectedSource string) (Manifest, error) {
	return m.stagePackage(ctx, path, func(stage string, next Manifest) error {
		m.mu.Lock()
		defer m.mu.Unlock()
		current, installed := m.manifests[id]
		if !installed {
			return ErrUnsupported
		}
		if next.ID != id {
			return fmt.Errorf("plugin: package ID %q does not match installed plugin %q", next.ID, id)
		}
		currentSource := m.updateSourceLocked(id)
		if expectedSource != "" && currentSource != expectedSource {
			return errors.New("plugin: the saved update source changed; try again")
		}
		if source == "" {
			source = currentSource
		}
		cmp, err := comparePluginVersions(next.Version, current.Version)
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := writeUpdateSource(stage, source); err != nil {
			return err
		}
		if cmp <= 0 {
			// A legacy installation can learn its source from a matching release
			// without replacing the package with the same version.
			if cmp == 0 && currentSource == "" && source != "" && !m.bundled[id] {
				if err := os.Rename(filepath.Join(stage, pluginUpdateSourceFile), filepath.Join(m.paths[id], pluginUpdateSourceFile)); err != nil {
					return err
				}
			}
			return fmt.Errorf("%w (installed %s, package %s)", ErrNoPluginUpdate, current.Version, next.Version)
		}
		return m.replacePackageLocked(ctx, stage, next)
	})
}

func (m *Manager) replacePackageLocked(ctx context.Context, stage string, next Manifest) error {
	id := next.ID
	root := filepath.Join(m.dir, "plugins")
	dest := filepath.Join(root, id)
	bundled, enabled := m.bundled[id], m.enabled[id]
	old := m.manifests[id]
	oldPath := m.paths[id]
	txn, err := os.MkdirTemp(root, ".update-")
	if err != nil {
		return err
	}
	keepBackup := false
	defer func() {
		if !keepBackup {
			_ = os.RemoveAll(txn)
		}
	}()

	oldClient := m.clients[id]
	if oldClient != nil {
		// Windows cannot rename a directory containing an open executable.
		_ = oldClient.Close()
		delete(m.clients, id)
	}
	restartOld := func() error {
		if oldClient == nil {
			return nil
		}
		client, err := startPlugin(context.WithoutCancel(ctx), old, oldPath, m.dir)
		if err == nil {
			m.clients[id] = client
		}
		return err
	}

	state := filepath.Join(m.dir, "plugin-data", id)
	validationDir := filepath.Join(txn, "validation")
	snapshot := filepath.Join(validationDir, "plugin-data", id)
	hadState, err := copyPluginState(ctx, state, snapshot)
	if err != nil {
		return errors.Join(fmt.Errorf("plugin: prepare update data: %w", err), restartOld())
	}
	if err := ctx.Err(); err != nil {
		return errors.Join(err, restartOld())
	}

	backup := filepath.Join(txn, "package")
	if !bundled {
		st, err := os.Lstat(dest)
		if err != nil {
			return errors.Join(err, restartOld())
		}
		if !st.IsDir() {
			return errors.Join(errors.New("plugin: unsafe installed package directory"), restartOld())
		}
		if err := os.Rename(dest, backup); err != nil {
			return errors.Join(err, restartOld())
		}
	}
	rollback := func(cause error, installed, restoreState bool) error {
		var rollbackErr error
		if installed {
			rollbackErr = os.Rename(dest, stage)
		}
		if rollbackErr == nil && !bundled {
			rollbackErr = os.Rename(backup, dest)
		}
		if restoreState {
			stateErr := os.Rename(state, filepath.Join(txn, "failed-data"))
			if errors.Is(stateErr, fs.ErrNotExist) {
				stateErr = nil
			}
			if stateErr == nil && hadState {
				stateErr = os.Rename(snapshot, state)
			}
			rollbackErr = errors.Join(rollbackErr, stateErr)
		}
		if rollbackErr != nil {
			keepBackup = true
			return errors.Join(cause, fmt.Errorf("plugin: update rollback failed; backup retained in %s: %w", txn, rollbackErr))
		}
		return errors.Join(cause, m.reloadLocked(), restartOld())
	}
	if err := os.Rename(stage, dest); err != nil {
		return rollback(err, false, false)
	}
	if enabled {
		validationDir = m.dir
	}
	client, err := startPlugin(ctx, next, dest, validationDir)
	if err != nil {
		return rollback(fmt.Errorf("plugin: updated executable failed to start: %w", err), true, enabled)
	}
	if !enabled {
		_ = client.Close()
	}
	err = ctx.Err()
	if err == nil {
		err = m.reloadLocked()
	}
	if err == nil && bundled {
		// An updated bundled plugin is now a user package. Persist its current
		// enabled state, including when it came from the bundled default.
		err = m.saveEnabled()
	}
	if err != nil {
		_ = client.Close()
		return rollback(err, true, enabled)
	}
	if enabled {
		m.clients[id] = client
	}
	return nil
}

func copyPluginState(ctx context.Context, src, dst string) (bool, error) {
	if _, err := os.Lstat(src); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	err := filepath.WalkDir(src, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, info.Mode().Perm())
		}
		if !info.Mode().IsRegular() || rel == "." {
			return fmt.Errorf("plugin: cannot copy data entry %q", rel)
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, info.Mode().Perm())
		if err != nil {
			return err
		}
		_, err = io.Copy(out, in)
		return errors.Join(err, out.Close())
	})
	return true, err
}
