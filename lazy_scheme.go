package ydb

import (
	context "context"
	"fmt"
	"github.com/ydb-platform/ydb-go-sdk/v3/internal/errors"
	"github.com/ydb-platform/ydb-go-sdk/v3/internal/scheme"
	"github.com/ydb-platform/ydb-go-sdk/v3/internal/table"
	"path"
	"strings"
	"sync"
)

type lazyScheme struct {
	db     dbWithTable
	client scheme.Scheme
	once   sync.Once
}

func (s *lazyScheme) Close(ctx context.Context) error {
	s.init()
	return s.client.Close(ctx)
}

func (s *lazyScheme) init() {
	s.once.Do(func() {
		s.client = scheme.New(s.db)
	})
}

func (c *lazyScheme) EnsurePathExists(ctx context.Context, path string) error {
	for i := len(c.db.Name()); i < len(path); i++ {
		x := strings.IndexByte(path[i:], '/')
		if x == -1 {
			x = len(path[i:]) - 1
		}
		i += x
		sub := path[:i+1]
		info, err := c.DescribePath(ctx, sub)
		operr, ok := err.(*errors.OpError)
		if ok && operr.Reason == errors.StatusSchemeError {
			err = c.MakeDirectory(ctx, sub)
		}
		if err != nil {
			return err
		}
		if ok {
			continue
		}
		switch info.Type {
		case
			scheme.EntryDatabase,
			scheme.EntryDirectory:
			// OK
		default:
			return fmt.Errorf(
				"entry %q exists but it is a %s",
				sub, info.Type,
			)
		}
	}

	return nil
}

func (c *lazyScheme) CleanupDatabase(ctx context.Context, prefix string, names ...string) error {
	filter := make(map[string]struct{}, len(names))
	for _, n := range names {
		filter[n] = struct{}{}
	}
	var list func(int, string) error
	list = func(i int, p string) error {
		dir, err := c.ListDirectory(ctx, p)
		operr, ok := err.(*errors.OpError)
		if ok && operr.Reason == errors.StatusSchemeError {
			return nil
		}
		if err != nil {
			return err
		}
		for _, child := range dir.Children {
			if _, has := filter[child.Name]; !has {
				continue
			}
			pt := path.Join(p, child.Name)
			switch child.Type {
			case scheme.EntryDirectory:
				if err := list(i+1, pt); err != nil {
					return err
				}
				if err := c.RemoveDirectory(ctx, pt); err != nil {
					return err
				}

			case scheme.EntryTable:
				err, _ = c.db.Table().Retry(ctx, false, func(ctx context.Context, session *table.Session) (err error) {
					return session.DropTable(ctx, pt)
				})
				if err != nil {
					return err
				}

			default:

			}
		}
		return nil
	}
	return list(0, prefix)
}

func (s *lazyScheme) DescribePath(ctx context.Context, path string) (e scheme.Entry, err error) {
	s.init()
	return s.client.DescribePath(ctx, path)
}

func (s *lazyScheme) MakeDirectory(ctx context.Context, path string) (err error) {
	s.init()
	return s.client.MakeDirectory(ctx, path)
}

func (s *lazyScheme) ListDirectory(ctx context.Context, path string) (d scheme.Directory, err error) {
	s.init()
	return s.client.ListDirectory(ctx, path)
}

func (s *lazyScheme) RemoveDirectory(ctx context.Context, path string) (err error) {
	s.init()
	return s.client.RemoveDirectory(ctx, path)
}

func newScheme(db dbWithTable) *lazyScheme {
	return &lazyScheme{
		db: db,
	}
}