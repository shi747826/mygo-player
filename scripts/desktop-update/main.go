// desktop-update prepares the signed manifest and archive consumed by MyGo.
package main

import (
	"archive/tar"
	"compress/gzip"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	log.SetFlags(0)
	args := os.Args[1:]
	var err error
	switch {
	case len(args) == 2 && args[0] == "keygen":
		err = keygen(args[1])
	case len(args) == 4:
		err = pack(args[0], args[1], args[2], args[3])
	default:
		err = errors.New("usage: desktop-update keygen KEY_FILE | PLATFORM VERSION APP_PATH OUTPUT_DIR")
	}
	if err != nil {
		log.Fatal(err)
	}
}

func keygen(filename string) error {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	_, writeErr := fmt.Fprintln(f, base64.StdEncoding.EncodeToString(private))
	if err := errors.Join(writeErr, f.Close()); err != nil {
		return err
	}
	fmt.Println(base64.StdEncoding.EncodeToString(public))
	return nil
}

func pack(platform, version, app, output string) error {
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(os.Getenv("MYGO_PLAYER_UPDATE_PRIVATE_KEY")))
	if err != nil || len(key) != ed25519.PrivateKeySize {
		return errors.New("invalid MYGO_PLAYER_UPDATE_PRIVATE_KEY")
	}
	private := ed25519.NewKeyFromSeed(key[:ed25519.SeedSize])
	public := base64.StdEncoding.EncodeToString(private.Public().(ed25519.PublicKey))
	if public != os.Getenv("MYGO_PLAYER_UPDATE_PUBLIC_KEY") {
		return errors.New("desktop update signing key does not match MYGO_PLAYER_UPDATE_PUBLIC_KEY")
	}
	stem := "update-" + platform
	name := stem + ".tar.gz"
	f, err := os.CreateTemp(output, ".update-*.tar.gz")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	digest := sha256.New()
	gz := gzip.NewWriter(io.MultiWriter(f, digest))
	archive := tar.NewWriter(gz)
	root := app
	if platform == "darwin-arm64" {
		root = filepath.Dir(app)
	}
	walkErr := filepath.WalkDir(app, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == root || (platform == "windows-amd64" && path == filepath.Join(root, "MicrosoftEdgeWebview2Setup.exe")) {
			return nil
		}
		return addFile(archive, root, path, d)
	})
	if err := errors.Join(walkErr, archive.Close(), gz.Close(), f.Close()); err != nil {
		return err
	}
	info, err := os.Stat(f.Name())
	if err != nil {
		return err
	}
	manifest := struct {
		Version   string `json:"version"`
		Date      string `json:"date"`
		URL       string `json:"url"`
		Size      int64  `json:"size"`
		Signature string `json:"signature"`
	}{version, time.Now().UTC().Format(time.RFC3339), name, info.Size(), base64.StdEncoding.EncodeToString(ed25519.Sign(private, digest.Sum(nil)))}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	final := filepath.Join(output, name)
	if err := os.Rename(f.Name(), final); err != nil {
		return err
	}
	if err := os.Chmod(final, 0644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(output, stem+".json"), append(data, '\n'), 0644)
}

func addFile(archive *tar.Writer, root, path string, d fs.DirEntry) error {
	info, err := d.Info()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() && !d.IsDir() {
		return fmt.Errorf("unsupported file in desktop update: %s", path)
	}
	header, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return err
	}
	header.Name = filepath.ToSlash(rel)
	header.Uid, header.Gid, header.Uname, header.Gname = 0, 0, "", ""
	if err = archive.WriteHeader(header); err != nil || d.IsDir() {
		return err
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(archive, f)
	return err
}
