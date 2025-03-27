package service

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"

	"github.com/bufbuild/connect-go"
	getter "github.com/hashicorp/go-getter"
	v1 "github.com/tierklinik-dobersberg/apis/gen/go/tkd/printing/v1"
)

func (svc *Service) resolveContent(document *v1.Document) (io.ReadCloser, int64, error) {
	switch v := document.Source.(type) {
	case *v1.Document_Data:
		return io.NopCloser(bytes.NewReader(v.Data)), int64(len(v.Data)), nil

	case *v1.Document_FilePath:
		if svc.providers.Storage == nil {
			return nil, 0, connect.NewError(connect.CodeUnavailable, fmt.Errorf("file_path support is not available"))
		}

		file, err := svc.providers.Storage.Open(v.FilePath)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil, 0, connect.NewError(connect.CodeNotFound, err)
			}

			return nil, 0, err
		}

		s, err := file.Stat()
		if err != nil {
			file.Close()
			return nil, 0, fmt.Errorf("failed to stat source file %q: %w", v.FilePath, err)
		}

		return file, s.Size(), nil

	case *v1.Document_Url:
		dst, err := os.CreateTemp(svc.providers.Config.StoragePath, document.Name+"-*")
		if err != nil {
			return nil, 0, fmt.Errorf("failed to create temporary file: %w", err)
		}
		dst.Close()

		if err := getter.GetFile(v.Url, dst.Name()); err != nil {
			return nil, 0, fmt.Errorf("failed to download document content: %w", err)
		}

		// finnally, open the temporary file with a delete-on-close wrapper
		f, err := newSelfDeletingFile(dst.Name())
		if err != nil {
			return nil, 0, err
		}

		stat, err := f.Stat()
		if err != nil {
			f.Close()
			return nil, 0, fmt.Errorf("failed to stat temporary file: %w", err)
		}

		return f, stat.Size(), nil

	default:
		return nil, 0, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid document source"))
	}
}

func newSelfDeletingFile(path string) (*selfDeletingFile, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	return &selfDeletingFile{
		path: path,
		File: f,
	}, nil
}

type selfDeletingFile struct {
	path string

	fs.File
}

func (sdf *selfDeletingFile) Close() error {
	err := sdf.File.Close()

	if err := os.Remove(sdf.path); err != nil {
		slog.Error("failed to delete temporary file", "path", sdf.path, "error", err)
	}

	return err
}
