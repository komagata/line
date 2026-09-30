package gui

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"net/url"
	"os"
	"path"
	"strings"
	"unicode"

	"golang.org/x/sys/unix"
)

const MaxFile = 20 << 20

func filePath(raw string) (string, error) {
	if len(raw) > 4096 {
		return "", errors.New("path too long")
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "file" || u.Host != "" || u.RawQuery != "" || u.Fragment != "" || u.Opaque != "" || !strings.HasPrefix(u.Path, "/") {
		return "", errors.New("invalid file URL")
	}
	p := u.Path
	for _, r := range p {
		if unicode.IsControl(r) || r >= 0x202a && r <= 0x202e || r >= 0x2066 && r <= 0x2069 || r == '\\' {
			return "", errors.New("unsafe path")
		}
	}
	return p, nil
}
func openParent(raw string) (int, string, error) {
	p, err := filePath(raw)
	if err != nil {
		return -1, "", err
	}
	parts := strings.Split(p, "/")
	if len(parts) < 2 {
		return -1, "", errors.New("missing name")
	}
	name := parts[len(parts)-1]
	if name == "" || name == "." || name == ".." || len(name) > 240 {
		return -1, "", errors.New("invalid name")
	}
	fd, err := unix.Open("/", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return -1, "", err
	}
	for _, part := range parts[1 : len(parts)-1] {
		if part == "" || part == "." || part == ".." {
			unix.Close(fd)
			return -1, "", errors.New("invalid directory")
		}
		next, err := unix.Openat(fd, part, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		unix.Close(fd)
		if err != nil {
			return -1, "", err
		}
		fd = next
	}
	return fd, name, nil
}
func snapshotFile(raw string) (*Attachment, error) {
	fd, name, err := openParent(raw)
	if err != nil {
		return nil, err
	}
	defer unix.Close(fd)
	child, err := unix.Openat(fd, name, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	f := os.NewFile(uintptr(child), name)
	defer f.Close()
	var before, after unix.Stat_t
	if unix.Fstat(child, &before) != nil || before.Mode&unix.S_IFMT != unix.S_IFREG || before.Size < 0 || before.Size > MaxFile {
		return nil, errors.New("not a bounded regular file")
	}
	data, err := io.ReadAll(io.LimitReader(f, MaxFile+1))
	if err != nil || len(data) > MaxFile || int64(len(data)) != before.Size {
		return nil, errors.New("file changed")
	}
	if unix.Fstat(child, &after) != nil || before.Size != after.Size || before.Mtim != after.Mtim || before.Ctim != after.Ctim {
		return nil, errors.New("file changed")
	}
	return &Attachment{ID: token(), Name: name, Size: len(data), Data: data}, nil
}
func publishFile(ctx context.Context, fd int, name string, data []byte, guards ...func(func() error) error) error {
	if len(data) > MaxFile {
		return errors.New("file too large")
	}
	temp := ".omarchy-line-" + token()
	child, err := unix.Openat(fd, temp, unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0600)
	if err != nil {
		return err
	}
	defer unix.Unlinkat(fd, temp, 0)
	f := os.NewFile(uintptr(child), temp)
	_, err = f.Write(data)
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	link := func() error { return unix.Linkat(fd, temp, fd, name, 0) }
	if len(guards) > 0 {
		return guards[0](link)
	}
	return link()
}
func sanitizedImage(data []byte, maxBytes, maxDimension int) string {
	if len(data) == 0 || len(data) > maxBytes {
		return ""
	}
	var mime string
	if bytes.HasPrefix(data, []byte{137, 80, 78, 71, 13, 10, 26, 10}) {
		mime = "image/png"
	} else if bytes.HasPrefix(data, []byte{255, 216}) {
		mime = "image/jpeg"
	} else {
		return ""
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || format != strings.TrimPrefix(mime, "image/") || config.Width < 1 || config.Height < 1 || config.Width > maxDimension || config.Height > maxDimension {
		return ""
	}
	img, format, err := image.Decode(bytes.NewReader(data))
	if err != nil || format != strings.TrimPrefix(mime, "image/") {
		return ""
	}
	var b bytes.Buffer
	if mime == "image/png" {
		err = png.Encode(&b, img)
	} else {
		err = jpeg.Encode(&b, img, &jpeg.Options{Quality: 90})
	}
	if err != nil || b.Len() > maxBytes {
		return ""
	}
	return fmt.Sprintf("data:%s;base64,%s", mime, base64.StdEncoding.EncodeToString(b.Bytes()))
}
func safeName(name string) string {
	name = path.Base(name)
	if name == "." || name == "/" || len(name) > 240 {
		return "file"
	}
	return name
}
